package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/auth"
	httpapi "github.com/vellus-ai/argoclaw/internal/http"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// testAuthResp mirrors the JSON shape of the auth response.
type testAuthResp struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         *store.User `json:"user"`
}

// --- in-memory stub of store.UserStore ---

type stubUserStore struct {
	users              map[string]*store.User // keyed by email
	sessions           map[string]*store.UserSession
	audit              []store.LoginAuditEntry
	history            map[uuid.UUID][]string // password history per user
	mustChangeCleared  bool                   // tracks whether ClearMustChangePassword was called
}

func newStubUserStore() *stubUserStore {
	return &stubUserStore{
		users:    make(map[string]*store.User),
		sessions: make(map[string]*store.UserSession),
		history:  make(map[uuid.UUID][]string),
	}
}

func (s *stubUserStore) CreateUser(_ context.Context, u *store.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	s.users[u.Email] = u
	return nil
}

func (s *stubUserStore) GetByEmail(_ context.Context, email string) (*store.User, error) {
	u, ok := s.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (s *stubUserStore) GetByID(_ context.Context, id uuid.UUID) (*store.User, error) {
	for _, u := range s.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (s *stubUserStore) UpdatePassword(_ context.Context, id uuid.UUID, hash string) error {
	for _, u := range s.users {
		if u.ID == id {
			u.PasswordHash = hash
			return nil
		}
	}
	return nil
}

func (s *stubUserStore) IncrementFailedAttempts(_ context.Context, id uuid.UUID, lockUntil *time.Time) (int, error) {
	for _, u := range s.users {
		if u.ID == id {
			u.FailedAttempts++
			if lockUntil != nil {
				u.LockedUntil = lockUntil
			}
			return u.FailedAttempts, nil
		}
	}
	return 0, nil
}

func (s *stubUserStore) ResetFailedAttempts(_ context.Context, id uuid.UUID) error {
	for _, u := range s.users {
		if u.ID == id {
			u.FailedAttempts = 0
			u.LockedUntil = nil
			return nil
		}
	}
	return nil
}

func (s *stubUserStore) UpdateLastLogin(_ context.Context, _ uuid.UUID) error { return nil }

func (s *stubUserStore) AddPasswordHistory(_ context.Context, id uuid.UUID, hash string) error {
	s.history[id] = append(s.history[id], hash)
	return nil
}

func (s *stubUserStore) GetPasswordHistory(_ context.Context, id uuid.UUID, limit int) ([]string, error) {
	h := s.history[id]
	if len(h) > limit {
		h = h[len(h)-limit:]
	}
	return h, nil
}

func (s *stubUserStore) CreateSession(_ context.Context, sess *store.UserSession) error {
	if sess.ID == uuid.Nil {
		sess.ID = uuid.New()
	}
	s.sessions[sess.RefreshToken] = sess
	return nil
}

func (s *stubUserStore) GetSessionByToken(_ context.Context, tokenHash string) (*store.UserSession, error) {
	sess, ok := s.sessions[tokenHash]
	if !ok || sess.Revoked {
		return nil, nil
	}
	return sess, nil
}

func (s *stubUserStore) RevokeSession(_ context.Context, id uuid.UUID) error {
	for k, sess := range s.sessions {
		if sess.ID == id {
			sess.Revoked = true
			s.sessions[k] = sess
			return nil
		}
	}
	return nil
}

func (s *stubUserStore) RevokeAllSessions(_ context.Context, _ uuid.UUID) error { return nil }
func (s *stubUserStore) CleanExpiredSessions(_ context.Context) error             { return nil }
func (s *stubUserStore) ClearMustChangePassword(_ context.Context, _ uuid.UUID) error {
	s.mustChangeCleared = true
	return nil
}

func (s *stubUserStore) LogAudit(_ context.Context, entry *store.LoginAuditEntry) error {
	s.audit = append(s.audit, *entry)
	return nil
}

// --- tests ---

const testJWTSecret = "test-jwt-secret-key-min-32-chars!!!"

func TestUserAuth_Register_Success(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"new@example.com","password":"Str0ng!Pass#99","display_name":"New User"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp testAuthResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("access_token is empty")
	}
	if resp.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}
	if resp.User == nil || resp.User.Email != "new@example.com" {
		t.Error("user email mismatch")
	}

	// Verify user created in store
	if _, ok := us.users["new@example.com"]; !ok {
		t.Error("user not created in store")
	}

	// Verify audit log
	if len(us.audit) < 1 || us.audit[0].Action != "register" {
		t.Error("expected register audit entry")
	}
}

func TestUserAuth_Register_DuplicateEmail(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	hash, _ := auth.HashPassword("Str0ng!Pass#99")
	us.users["existing@example.com"] = &store.User{
		ID: uuid.New(), Email: "existing@example.com", PasswordHash: hash,
		Role: "member", Status: "active",
	}

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"existing@example.com","password":"Str0ng!Pass#99","display_name":"Dup"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestUserAuth_Register_WeakPassword(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"new@example.com","password":"short","display_name":"Weak"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserAuth_Login_Success(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	password := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(password)
	us.users["user@example.com"] = &store.User{
		ID: uuid.New(), Email: "user@example.com", PasswordHash: hash,
		Role: "admin", Status: "active", TenantID: ptrUUID(uuid.New()),
	}

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"user@example.com","password":"Str0ng!Pass#99"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp testAuthResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("access_token is empty")
	}

	// Validate the access token
	claims, err := auth.ValidateAccessToken(resp.AccessToken, testJWTSecret)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("claims.Email = %q, want user@example.com", claims.Email)
	}

	// Verify audit log has login_success
	found := false
	for _, a := range us.audit {
		if a.Action == "login_success" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected login_success audit entry")
	}
}

func TestUserAuth_Login_WrongPassword(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	hash, _ := auth.HashPassword("Str0ng!Pass#99")
	us.users["user@example.com"] = &store.User{
		ID: uuid.New(), Email: "user@example.com", PasswordHash: hash,
		Role: "member", Status: "active",
	}

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"user@example.com","password":"Wrong!Pass#99X"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// Verify failed attempt counter incremented
	if us.users["user@example.com"].FailedAttempts != 1 {
		t.Errorf("FailedAttempts = %d, want 1", us.users["user@example.com"].FailedAttempts)
	}
}

func TestUserAuth_Login_NonExistentUser(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"nobody@example.com","password":"Str0ng!Pass#99"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should return 401 (same as wrong password — no enumeration)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestUserAuth_Login_Lockout(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	hash, _ := auth.HashPassword("Str0ng!Pass#99")
	lockTime := time.Now().Add(30 * time.Minute)
	us.users["locked@example.com"] = &store.User{
		ID: uuid.New(), Email: "locked@example.com", PasswordHash: hash,
		Role: "member", Status: "active", FailedAttempts: 5, LockedUntil: &lockTime,
	}

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"locked@example.com","password":"Str0ng!Pass#99"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusTooManyRequests, w.Body.String())
	}
}

func TestUserAuth_Login_DisabledAccount(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	password := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(password)
	us.users["disabled@example.com"] = &store.User{
		ID: uuid.New(), Email: "disabled@example.com", PasswordHash: hash,
		Role: "member", Status: "disabled",
	}

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"disabled@example.com","password":"Str0ng!Pass#99"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusForbidden, w.Body.String())
	}

	// Verify audit log
	found := false
	for _, a := range us.audit {
		if a.Action == "login_inactive" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected login_inactive audit entry")
	}
}

func TestUserAuth_Login_SuspendedAccount(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	password := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(password)
	us.users["suspended@example.com"] = &store.User{
		ID: uuid.New(), Email: "suspended@example.com", PasswordHash: hash,
		Role: "member", Status: "suspended",
	}

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"suspended@example.com","password":"Str0ng!Pass#99"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestUserAuth_Login_PendingAccount(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	password := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(password)
	us.users["pending@example.com"] = &store.User{
		ID: uuid.New(), Email: "pending@example.com", PasswordHash: hash,
		Role: "member", Status: "pending",
	}

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"pending@example.com","password":"Str0ng!Pass#99"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestUserAuth_Logout_Revokes(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	hash, _ := auth.HashPassword("Str0ng!Pass#99")
	user := &store.User{
		ID: uuid.New(), Email: "user@example.com", PasswordHash: hash,
		Role: "member", Status: "active",
	}
	us.users[user.Email] = user

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Login first
	loginBody := `{"email":"user@example.com","password":"Str0ng!Pass#99"}`
	loginReq := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	mux.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginW.Code, http.StatusOK)
	}

	var loginResp testAuthResp
	json.Unmarshal(loginW.Body.Bytes(), &loginResp)

	// Logout
	logoutBody := `{"refresh_token":"` + loginResp.RefreshToken + `"}`
	logoutReq := httptest.NewRequest("POST", "/v1/auth/logout", bytes.NewBufferString(logoutBody))
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutW := httptest.NewRecorder()
	mux.ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d; body = %s", logoutW.Code, http.StatusOK, logoutW.Body.String())
	}
}

// --- Refresh tests ---

func TestUserAuth_Refresh_Success(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	password := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(password)
	user := &store.User{
		ID: uuid.New(), Email: "user@example.com", PasswordHash: hash,
		Role: "admin", Status: "active", TenantID: ptrUUID(uuid.New()),
	}
	us.users[user.Email] = user

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Login first to get a refresh token
	loginBody := `{"email":"user@example.com","password":"Str0ng!Pass#99"}`
	loginReq := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	mux.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginW.Code, http.StatusOK)
	}

	var loginResp testAuthResp
	json.Unmarshal(loginW.Body.Bytes(), &loginResp)

	// Refresh
	refreshBody := `{"refresh_token":"` + loginResp.RefreshToken + `"}`
	refreshReq := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBufferString(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshW := httptest.NewRecorder()
	mux.ServeHTTP(refreshW, refreshReq)

	if refreshW.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want %d; body = %s", refreshW.Code, http.StatusOK, refreshW.Body.String())
	}

	var refreshResp testAuthResp
	if err := json.Unmarshal(refreshW.Body.Bytes(), &refreshResp); err != nil {
		t.Fatalf("unmarshal refresh: %v", err)
	}
	if refreshResp.AccessToken == "" {
		t.Error("new access_token is empty")
	}
	if refreshResp.RefreshToken == "" {
		t.Error("new refresh_token is empty")
	}
	// New refresh token should differ from original (rotation)
	if refreshResp.RefreshToken == loginResp.RefreshToken {
		t.Error("refresh token was not rotated")
	}
}

func TestUserAuth_Refresh_InvalidToken(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"refresh_token":"nonexistent-token-value"}`
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestUserAuth_Refresh_RevokedToken(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	password := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(password)
	user := &store.User{
		ID: uuid.New(), Email: "user@example.com", PasswordHash: hash,
		Role: "member", Status: "active",
	}
	us.users[user.Email] = user

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Login
	loginBody := `{"email":"user@example.com","password":"Str0ng!Pass#99"}`
	loginReq := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	mux.ServeHTTP(loginW, loginReq)

	var loginResp testAuthResp
	json.Unmarshal(loginW.Body.Bytes(), &loginResp)

	// Use refresh once (this revokes the old session)
	refreshBody := `{"refresh_token":"` + loginResp.RefreshToken + `"}`
	refreshReq := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBufferString(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshW := httptest.NewRecorder()
	mux.ServeHTTP(refreshW, refreshReq)

	if refreshW.Code != http.StatusOK {
		t.Fatalf("first refresh failed: %d", refreshW.Code)
	}

	// Reuse old token — should fail (session revoked)
	reuseReq := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBufferString(refreshBody))
	reuseReq.Header.Set("Content-Type", "application/json")
	reuseW := httptest.NewRecorder()
	mux.ServeHTTP(reuseW, reuseReq)

	if reuseW.Code != http.StatusUnauthorized {
		t.Fatalf("reuse status = %d, want %d (revoked token should fail)", reuseW.Code, http.StatusUnauthorized)
	}
}

// --- Edge case tests ---

func TestUserAuth_Register_MalformedJSON(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserAuth_Register_EmptyBody(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserAuth_Register_MissingEmail(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"password":"Str0ng!Pass#99","display_name":"No Email"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserAuth_Register_EmailNormalization(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":" USER@EXAMPLE.COM ","password":"Str0ng!Pass#99","display_name":"Upper"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	// Verify email was normalized to lowercase
	if _, ok := us.users["user@example.com"]; !ok {
		t.Error("email was not normalized to lowercase")
	}
}

func TestUserAuth_Login_MalformedJSON(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserAuth_Refresh_MalformedJSON(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBufferString("broken{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserAuth_Logout_MalformedJSON(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/auth/logout", bytes.NewBufferString("{{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- Rate limiting tests ---

func TestUserAuth_Login_RateLimited(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	password := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(password)
	us.users["user@example.com"] = &store.User{
		ID: uuid.New(), Email: "user@example.com", PasswordHash: hash,
		Role: "member", Status: "active",
	}

	// 2 RPM, burst 1 — very restrictive for testing
	rl := httpapi.NewAuthRateLimiter(2, 2, 20)
	h := httpapi.NewUserAuthHandler(us, testJWTSecret, httpapi.WithRateLimiter(rl))
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"email":"user@example.com","password":"Str0ng!Pass#99"}`

	// First request: should succeed
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Rapid subsequent requests should eventually be rate limited
	rateLimited := false
	for i := 0; i < 10; i++ {
		req = httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			rateLimited = true
			// Check Retry-After header
			if w.Header().Get("Retry-After") == "" {
				t.Error("429 response missing Retry-After header")
			}
			break
		}
	}
	if !rateLimited {
		t.Error("expected 429 after rapid login requests")
	}
}

func TestUserAuth_Register_RateLimited(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()

	// 1 RPM, burst 1 — very restrictive
	rl := httpapi.NewAuthRateLimiter(2, 1, 20)
	h := httpapi.NewUserAuthHandler(us, testJWTSecret, httpapi.WithRateLimiter(rl))
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rateLimited := false
	for i := 0; i < 10; i++ {
		email := "user" + string(rune('a'+i)) + "@example.com"
		body := `{"email":"` + email + `","password":"Str0ng!Pass#99","display_name":"User"}`
		req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			rateLimited = true
			break
		}
	}
	if !rateLimited {
		t.Error("expected 429 after rapid register requests")
	}
}

// --- Change Password tests ---

// generateTestJWT creates a JWT for testing without going through login.
// This avoids the issueTokens side effect that clears user.PasswordHash
// on the shared pointer (which would corrupt the in-memory stub).
func generateTestJWT(t *testing.T, user *store.User) string {
	t.Helper()
	tenantID := ""
	if user.TenantID != nil {
		tenantID = user.TenantID.String()
	}
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:             user.ID.String(),
		Email:              user.Email,
		TenantID:           tenantID,
		Role:               user.Role,
		MustChangePassword: user.MustChangePassword,
	}, testJWTSecret)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	return token
}

func TestChangePassword_Success(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	currentPassword := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(currentPassword)
	tenantID := uuid.New()
	user := &store.User{
		ID: uuid.New(), Email: "cp@example.com", PasswordHash: hash,
		Role: "admin", Status: "active", TenantID: ptrUUID(tenantID),
	}
	us.users[user.Email] = user

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	accessToken := generateTestJWT(t, user)

	// Change password
	cpBody := `{"current_password":"Str0ng!Pass#99","new_password":"N3w!Secure#Pass1"}`
	cpReq := httptest.NewRequest("POST", "/v1/auth/change-password", bytes.NewBufferString(cpBody))
	cpReq.Header.Set("Content-Type", "application/json")
	cpReq.Header.Set("Authorization", "Bearer "+accessToken)
	cpW := httptest.NewRecorder()
	mux.ServeHTTP(cpW, cpReq)

	if cpW.Code != http.StatusOK {
		t.Fatalf("change-password status = %d, want %d; body = %s", cpW.Code, http.StatusOK, cpW.Body.String())
	}

	// Verify new tokens are returned
	var cpResp testAuthResp
	if err := json.Unmarshal(cpW.Body.Bytes(), &cpResp); err != nil {
		t.Fatalf("unmarshal change-password response: %v", err)
	}
	if cpResp.AccessToken == "" {
		t.Error("access_token is empty after change-password")
	}
	if cpResp.RefreshToken == "" {
		t.Error("refresh_token is empty after change-password")
	}

	// Verify audit log has password_change entry
	found := false
	for _, a := range us.audit {
		if a.Action == "password_change" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected password_change audit entry")
	}
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	hash, _ := auth.HashPassword("Str0ng!Pass#99")
	tenantID := uuid.New()
	user := &store.User{
		ID: uuid.New(), Email: "cpwrong@example.com", PasswordHash: hash,
		Role: "member", Status: "active", TenantID: ptrUUID(tenantID),
	}
	us.users[user.Email] = user

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	accessToken := generateTestJWT(t, user)

	// Change password with wrong current password
	cpBody := `{"current_password":"Wrong!Pass#999","new_password":"N3w!Secure#Pass1"}`
	cpReq := httptest.NewRequest("POST", "/v1/auth/change-password", bytes.NewBufferString(cpBody))
	cpReq.Header.Set("Content-Type", "application/json")
	cpReq.Header.Set("Authorization", "Bearer "+accessToken)
	cpW := httptest.NewRecorder()
	mux.ServeHTTP(cpW, cpReq)

	if cpW.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", cpW.Code, http.StatusUnauthorized, cpW.Body.String())
	}
}

func TestChangePassword_InvalidNewPassword(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	hash, _ := auth.HashPassword("Str0ng!Pass#99")
	tenantID := uuid.New()
	user := &store.User{
		ID: uuid.New(), Email: "cpweak@example.com", PasswordHash: hash,
		Role: "member", Status: "active", TenantID: ptrUUID(tenantID),
	}
	us.users[user.Email] = user

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	accessToken := generateTestJWT(t, user)

	// Change password with weak new password
	cpBody := `{"current_password":"Str0ng!Pass#99","new_password":"short"}`
	cpReq := httptest.NewRequest("POST", "/v1/auth/change-password", bytes.NewBufferString(cpBody))
	cpReq.Header.Set("Content-Type", "application/json")
	cpReq.Header.Set("Authorization", "Bearer "+accessToken)
	cpW := httptest.NewRecorder()
	mux.ServeHTTP(cpW, cpReq)

	if cpW.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", cpW.Code, http.StatusBadRequest, cpW.Body.String())
	}
}

func TestChangePassword_NoAuth(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	cpBody := `{"current_password":"Str0ng!Pass#99","new_password":"N3w!Secure#Pass1"}`
	cpReq := httptest.NewRequest("POST", "/v1/auth/change-password", bytes.NewBufferString(cpBody))
	cpReq.Header.Set("Content-Type", "application/json")
	// No Authorization header
	cpW := httptest.NewRecorder()
	mux.ServeHTTP(cpW, cpReq)

	if cpW.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", cpW.Code, http.StatusUnauthorized, cpW.Body.String())
	}
}

func TestChangePassword_PasswordReuse(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	currentPassword := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(currentPassword)
	userID := uuid.New()
	tenantID := uuid.New()
	user := &store.User{
		ID: userID, Email: "cpreuse@example.com", PasswordHash: hash,
		Role: "member", Status: "active", TenantID: ptrUUID(tenantID),
	}
	us.users[user.Email] = user

	// Add current password hash to history so reuse is detected
	us.history[userID] = []string{hash}

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	accessToken := generateTestJWT(t, user)

	// Try to change password to the same current password (reuse)
	cpBody := `{"current_password":"Str0ng!Pass#99","new_password":"Str0ng!Pass#99"}`
	cpReq := httptest.NewRequest("POST", "/v1/auth/change-password", bytes.NewBufferString(cpBody))
	cpReq.Header.Set("Content-Type", "application/json")
	cpReq.Header.Set("Authorization", "Bearer "+accessToken)
	cpW := httptest.NewRecorder()
	mux.ServeHTTP(cpW, cpReq)

	if cpW.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", cpW.Code, http.StatusBadRequest, cpW.Body.String())
	}
}

func TestChangePassword_ClearsMustChangeFlag(t *testing.T) {
	t.Parallel()
	us := newStubUserStore()
	currentPassword := "Str0ng!Pass#99"
	hash, _ := auth.HashPassword(currentPassword)
	tenantID := uuid.New()
	user := &store.User{
		ID: uuid.New(), Email: "cpmust@example.com", PasswordHash: hash,
		Role: "member", Status: "active", TenantID: ptrUUID(tenantID),
		MustChangePassword: true,
	}
	us.users[user.Email] = user

	h := httpapi.NewUserAuthHandler(us, testJWTSecret)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	accessToken := generateTestJWT(t, user)

	// Change password
	cpBody := `{"current_password":"Str0ng!Pass#99","new_password":"N3w!Secure#Pass1"}`
	cpReq := httptest.NewRequest("POST", "/v1/auth/change-password", bytes.NewBufferString(cpBody))
	cpReq.Header.Set("Content-Type", "application/json")
	cpReq.Header.Set("Authorization", "Bearer "+accessToken)
	cpW := httptest.NewRecorder()
	mux.ServeHTTP(cpW, cpReq)

	if cpW.Code != http.StatusOK {
		t.Fatalf("change-password status = %d, want %d; body = %s", cpW.Code, http.StatusOK, cpW.Body.String())
	}

	// Verify ClearMustChangePassword was called
	if !us.mustChangeCleared {
		t.Error("ClearMustChangePassword was not called")
	}

	// Verify the new token does NOT have MustChangePassword flag
	var cpResp testAuthResp
	if err := json.Unmarshal(cpW.Body.Bytes(), &cpResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	claims, err := auth.ValidateAccessToken(cpResp.AccessToken, testJWTSecret)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.MustChangePassword {
		t.Error("MustChangePassword should be false in new token after password change")
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }
