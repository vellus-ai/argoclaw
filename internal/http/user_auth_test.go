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
	users    map[string]*store.User // keyed by email
	sessions map[string]*store.UserSession
	audit    []store.LoginAuditEntry
	history  map[uuid.UUID][]string // password history per user
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

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }
