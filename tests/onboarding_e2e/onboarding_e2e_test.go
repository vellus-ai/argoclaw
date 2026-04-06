//go:build integration

package onboarding_e2e_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vellus-ai/argoclaw/internal/auth"
	httpapi "github.com/vellus-ai/argoclaw/internal/http"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
	"github.com/vellus-ai/argoclaw/internal/testutil"
)

// --- helpers ---

func setupE2E(t *testing.T) (*sql.DB, uuid.UUID) {
	t.Helper()
	db := testutil.SetupDB(t)
	slug := "e2e-onb-" + uuid.Must(uuid.NewV7()).String()[:8]
	tenantID := testutil.CreateTestTenant(t, db, slug, "E2E Onboarding")
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM setup_progress WHERE tenant_id = $1", tenantID)
		db.ExecContext(context.Background(), "DELETE FROM tenant_branding WHERE tenant_id = $1", tenantID)
		db.ExecContext(context.Background(), "DELETE FROM users WHERE tenant_id = $1", tenantID)
	})
	return db, tenantID
}

func createTestUser(t *testing.T, db *sql.DB, tenantID uuid.UUID, email, password string, mustChange bool) *store.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &store.User{
		TenantID:           &tenantID,
		Email:              email,
		PasswordHash:       hash,
		DisplayName:        "E2E User",
		Role:               "owner",
		Status:             "active",
		MustChangePassword: mustChange,
	}
	us := pg.NewPGUserStore(db)
	if err := us.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	_ = us.AddPasswordHistory(context.Background(), user.ID, hash)
	return user
}

const jwtSecret = "e2e-test-jwt-secret-min-32-chars!!!"

func issueJWT(t *testing.T, user *store.User) string {
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
	}, jwtSecret)
	if err != nil {
		t.Fatalf("issue JWT: %v", err)
	}
	return token
}

// =============================================================================
// E2E: Onboarding Store Full Flow
// =============================================================================

func TestE2E_OnboardingFullFlow(t *testing.T) {
	db, tenantID := setupE2E(t)
	onbStore := pg.NewPGOnboardingStore(db)
	tid := tenantID.String()
	ctx := testutil.TenantCtx(tenantID)

	// Step 1: initial status should be empty
	status, err := onbStore.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("initial status: %v", err)
	}
	if status["onboarding_complete"] != false {
		t.Fatalf("expected onboarding_complete=false initially")
	}
	if status["workspace_configured"] != false {
		t.Fatalf("expected workspace_configured=false initially")
	}

	// Step 2: configure workspace
	for k, v := range map[string]any{
		"account_type": "business",
		"account_name": "E2E Corp",
		"industry":     "technology",
		"team_size":    "small",
	} {
		if err := onbStore.UpdateTenantSettings(ctx, tid, k, v); err != nil {
			t.Fatalf("UpdateTenantSettings(%s): %v", k, err)
		}
	}

	// Step 3: set branding
	if err := onbStore.UpdateTenantBranding(ctx, tid, "#1E40AF", "E2E Product"); err != nil {
		t.Fatalf("UpdateTenantBranding: %v", err)
	}

	// Step 4: verify status after configuration
	status, err = onbStore.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("status after config: %v", err)
	}
	if status["workspace_configured"] != true {
		t.Errorf("workspace_configured should be true after configure")
	}
	if status["branding_set"] != true {
		t.Errorf("branding_set should be true after branding")
	}
	if status["account_type"] != "business" {
		t.Errorf("account_type = %v, want business", status["account_type"])
	}
	if status["primary_color"] != "#1E40AF" {
		t.Errorf("primary_color = %v, want #1E40AF", status["primary_color"])
	}
	if status["onboarding_complete"] != false {
		t.Errorf("onboarding_complete should still be false")
	}

	// Step 5: complete onboarding
	if err := onbStore.CompleteOnboarding(ctx, tid); err != nil {
		t.Fatalf("CompleteOnboarding: %v", err)
	}

	// Step 6: verify completion
	status, err = onbStore.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("final status: %v", err)
	}
	if status["onboarding_complete"] != true {
		t.Errorf("onboarding_complete should be true")
	}
	if _, ok := status["completed_at"]; !ok {
		t.Errorf("completed_at should be set")
	}
}

// =============================================================================
// E2E: Tenant Isolation
// =============================================================================

func TestE2E_TenantIsolation(t *testing.T) {
	db, tenantA := setupE2E(t)
	slugB := "e2e-iso-b-" + uuid.Must(uuid.NewV7()).String()[:8]
	tenantB := testutil.CreateTestTenant(t, db, slugB, "E2E Tenant B")
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM setup_progress WHERE tenant_id = $1", tenantB)
		db.ExecContext(context.Background(), "DELETE FROM tenant_branding WHERE tenant_id = $1", tenantB)
	})

	onbStore := pg.NewPGOnboardingStore(db)
	ctxA := testutil.TenantCtx(tenantA)
	ctxB := testutil.TenantCtx(tenantB)

	// Configure tenant A
	if err := onbStore.UpdateTenantSettings(ctxA, tenantA.String(), "account_type", "business"); err != nil {
		t.Fatalf("A settings: %v", err)
	}
	if err := onbStore.UpdateTenantBranding(ctxA, tenantA.String(), "#FF0000", "TenantA"); err != nil {
		t.Fatalf("A branding: %v", err)
	}
	if err := onbStore.CompleteOnboarding(ctxA, tenantA.String()); err != nil {
		t.Fatalf("A complete: %v", err)
	}

	// Tenant B should be unaffected
	statusB, err := onbStore.GetOnboardingStatus(ctxB, tenantB.String())
	if err != nil {
		t.Fatalf("B status: %v", err)
	}
	if statusB["onboarding_complete"] != false {
		t.Errorf("tenant B should not be completed by tenant A's actions")
	}
	if statusB["workspace_configured"] != false {
		t.Errorf("tenant B should not have workspace configured")
	}
	if statusB["branding_set"] != false {
		t.Errorf("tenant B should not have branding set")
	}
}

// =============================================================================
// E2E: Change Password via HTTP
// =============================================================================

func TestE2E_ChangePassword(t *testing.T) {
	db, tenantID := setupE2E(t)

	email := fmt.Sprintf("e2e-%s@test.com", uuid.Must(uuid.NewV7()).String()[:8])
	oldPass := "OldPassword1!secure"
	newPass := "NewPassword2@better"

	user := createTestUser(t, db, tenantID, email, oldPass, true)

	us := pg.NewPGUserStore(db)
	h := httpapi.NewUserAuthHandler(us, jwtSecret)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	token := issueJWT(t, user)

	// Test: change password successfully
	body := fmt.Sprintf(`{"current_password":"%s","new_password":"%s"}`, oldPass, newPass)
	req := httptest.NewRequest("POST", "/v1/auth/change-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("change-password: status=%d, body=%s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := resp["access_token"]; !ok {
		t.Error("response should contain access_token")
	}
	if _, ok := resp["refresh_token"]; !ok {
		t.Error("response should contain refresh_token")
	}

	// Verify: must_change_password flag cleared
	updatedUser, err := us.GetByID(context.Background(), user.ID)
	if err != nil || updatedUser == nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updatedUser.MustChangePassword {
		t.Error("must_change_password should be false after password change")
	}

	// Verify: can login with new password
	if !auth.VerifyPassword(newPass, updatedUser.PasswordHash) {
		t.Error("new password should verify successfully")
	}
}

func TestE2E_ChangePassword_WrongCurrent(t *testing.T) {
	db, tenantID := setupE2E(t)

	email := fmt.Sprintf("e2e-%s@test.com", uuid.Must(uuid.NewV7()).String()[:8])
	user := createTestUser(t, db, tenantID, email, "Correct1!password", false)

	us := pg.NewPGUserStore(db)
	h := httpapi.NewUserAuthHandler(us, jwtSecret)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	token := issueJWT(t, user)

	body := `{"current_password":"Wrong1!password","new_password":"NewPass1!secure"}`
	req := httptest.NewRequest("POST", "/v1/auth/change-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong current password: status=%d, want 401", w.Code)
	}
}
