package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// UserAuthHandler manages user registration, login, and token refresh.
type UserAuthHandler struct {
	users       store.UserStore
	jwtSecret   string
	rateLimiter *AuthRateLimiter
}

// NewUserAuthHandler creates a new auth handler.
// rateLimiter is optional — pass nil to disable rate limiting.
func NewUserAuthHandler(users store.UserStore, jwtSecret string, opts ...UserAuthOption) *UserAuthHandler {
	h := &UserAuthHandler{users: users, jwtSecret: jwtSecret}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// UserAuthOption configures optional settings for UserAuthHandler.
type UserAuthOption func(*UserAuthHandler)

// WithRateLimiter sets the rate limiter for auth endpoints.
func WithRateLimiter(rl *AuthRateLimiter) UserAuthOption {
	return func(h *UserAuthHandler) {
		h.rateLimiter = rl
	}
}

// RegisterRoutes adds auth endpoints to the mux.
func (h *UserAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	register := h.handleRegister
	login := h.handleLogin
	refresh := h.handleRefresh

	if h.rateLimiter != nil {
		register = h.rateLimiter.WrapRegister(register)
		login = h.rateLimiter.WrapLogin(login)
		refresh = h.rateLimiter.WrapRefresh(refresh)
	}

	mux.HandleFunc("POST /v1/auth/register", register)
	mux.HandleFunc("POST /v1/auth/login", login)
	mux.HandleFunc("POST /v1/auth/refresh", refresh)
	mux.HandleFunc("POST /v1/auth/logout", h.handleLogout)
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	ExpiresIn    int        `json:"expires_in"` // seconds
	User         *store.User `json:"user"`
}

func (h *UserAuthHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		writeJSONError(w, http.StatusBadRequest, "email is required")
		return
	}

	// Validate password (PCI DSS)
	if err := auth.ValidatePassword(req.Password, req.Email); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check if email already exists
	existing, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		slog.Error("register: get user by email", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing != nil {
		writeJSONError(w, http.StatusConflict, "email already registered")
		return
	}

	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("register: hash password", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user := &store.User{
		Email:        req.Email,
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
		Role:         "member",
		Status:       "active",
	}

	if err := h.users.CreateUser(r.Context(), user); err != nil {
		slog.Error("register: create user", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Store initial password in history
	_ = h.users.AddPasswordHistory(r.Context(), user.ID, hash)

	// Audit
	_ = h.users.LogAudit(r.Context(), &store.LoginAuditEntry{
		UserID: &user.ID, Email: req.Email, Action: "register",
		IPAddress: clientIP(r), UserAgent: r.UserAgent(),
	})

	// Generate tokens
	resp, err := h.issueTokens(r, user)
	if err != nil {
		slog.Error("register: issue tokens", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *UserAuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		slog.Error("login: get user", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Constant-time response for non-existent users (prevent enumeration)
	if user == nil {
		auth.HashPassword(req.Password) // burn time to prevent timing attacks
		writeJSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Check lockout
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		_ = h.users.LogAudit(r.Context(), &store.LoginAuditEntry{
			UserID: &user.ID, Email: req.Email, Action: "login_locked",
			IPAddress: clientIP(r), UserAgent: r.UserAgent(),
		})
		writeJSONError(w, http.StatusTooManyRequests, "account locked — try again later")
		return
	}

	// Verify password
	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		lockUntil := (*time.Time)(nil)
		newCount, _ := h.users.IncrementFailedAttempts(r.Context(), user.ID, nil)
		if newCount >= auth.MaxFailedAttempts {
			t := time.Now().Add(time.Duration(auth.LockoutMinutes) * time.Minute)
			lockUntil = &t
			_, _ = h.users.IncrementFailedAttempts(r.Context(), user.ID, lockUntil)
			slog.Warn("security.account_locked", "email", req.Email, "attempts", newCount)
		}

		action := "login_failed"
		if lockUntil != nil {
			action = "lockout"
		}
		_ = h.users.LogAudit(r.Context(), &store.LoginAuditEntry{
			UserID: &user.ID, Email: req.Email, Action: action,
			IPAddress: clientIP(r), UserAgent: r.UserAgent(),
		})

		writeJSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Check account status before granting access
	if user.Status != "active" {
		_ = h.users.LogAudit(r.Context(), &store.LoginAuditEntry{
			UserID: &user.ID, Email: req.Email, Action: "login_inactive",
			IPAddress: clientIP(r), UserAgent: r.UserAgent(),
		})
		slog.Warn("security.login_inactive_account", "email", req.Email, "status", user.Status)
		writeJSONError(w, http.StatusForbidden, "account is not active")
		return
	}

	// Success — reset failed attempts
	_ = h.users.ResetFailedAttempts(r.Context(), user.ID)
	_ = h.users.UpdateLastLogin(r.Context(), user.ID)

	_ = h.users.LogAudit(r.Context(), &store.LoginAuditEntry{
		UserID: &user.ID, Email: req.Email, Action: "login_success",
		IPAddress: clientIP(r), UserAgent: r.UserAgent(),
	})

	resp, err := h.issueTokens(r, user)
	if err != nil {
		slog.Error("login: issue tokens", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UserAuthHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tokenHash := auth.HashRefreshToken(req.RefreshToken)
	session, err := h.users.GetSessionByToken(r.Context(), tokenHash)
	if err != nil {
		slog.Error("refresh: get session", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session == nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Revoke old session
	_ = h.users.RevokeSession(r.Context(), session.ID)

	// Get user
	user, err := h.users.GetByID(r.Context(), session.UserID)
	if err != nil || user == nil {
		writeJSONError(w, http.StatusUnauthorized, "user not found")
		return
	}

	// Reject refresh for inactive accounts
	if user.Status != "active" {
		writeJSONError(w, http.StatusForbidden, "account is not active")
		return
	}

	// Issue new tokens (rotation)
	resp, err := h.issueTokens(r, user)
	if err != nil {
		slog.Error("refresh: issue tokens", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UserAuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken != "" {
		tokenHash := auth.HashRefreshToken(req.RefreshToken)
		session, _ := h.users.GetSessionByToken(r.Context(), tokenHash)
		if session != nil {
			_ = h.users.RevokeSession(r.Context(), session.ID)
			_ = h.users.LogAudit(r.Context(), &store.LoginAuditEntry{
				UserID: &session.UserID, Email: "", Action: "logout",
				IPAddress: clientIP(r), UserAgent: r.UserAgent(),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- Helpers ---

func (h *UserAuthHandler) issueTokens(r *http.Request, user *store.User) (*authResponse, error) {
	tenantID := ""
	if user.TenantID != nil {
		tenantID = user.TenantID.String()
	}

	claims := auth.TokenClaims{
		UserID:   user.ID.String(),
		Email:    user.Email,
		TenantID: tenantID,
		Role:     user.Role,
	}

	accessToken, err := auth.GenerateAccessToken(claims, h.jwtSecret)
	if err != nil {
		return nil, err
	}

	rawRefresh, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	session := &store.UserSession{
		UserID:       user.ID,
		RefreshToken: refreshHash,
		UserAgent:    r.UserAgent(),
		IPAddress:    clientIP(r),
		ExpiresAt:    time.Now().Add(auth.RefreshTokenDuration),
	}
	if err := h.users.CreateSession(r.Context(), session); err != nil {
		return nil, err
	}

	// Clear sensitive fields before response
	user.PasswordHash = ""

	return &authResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int(auth.AccessTokenDuration.Seconds()),
		User:         user,
	}, nil
}

func writeJSONError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
