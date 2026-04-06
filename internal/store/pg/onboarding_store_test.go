//go:build integration

package pg_test

import (
	"testing"
	"testing/quick"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
	"github.com/vellus-ai/argoclaw/internal/testutil"
)

func setupOnboardingTest(t *testing.T) (*pg.PGOnboardingStore, uuid.UUID) {
	t.Helper()
	db := testutil.SetupDB(t)
	slug := "onb-" + uuid.Must(uuid.NewV7()).String()[:8]
	tenantID := testutil.CreateTestTenant(t, db, slug, "Onboarding Test")
	t.Cleanup(func() {
		db.ExecContext(testutil.TenantCtx(tenantID), "DELETE FROM setup_progress WHERE tenant_id = $1", tenantID)
		db.ExecContext(testutil.TenantCtx(tenantID), "DELETE FROM tenant_branding WHERE tenant_id = $1", tenantID)
	})
	return pg.NewPGOnboardingStore(db), tenantID
}

// --- UpdateTenantSettings ---

func TestUpdateTenantSettings_Basic(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()

	if err := store.UpdateTenantSettings(testutil.TenantCtx(tenantID), tid, "account_type", "business"); err != nil {
		t.Fatalf("UpdateTenantSettings: %v", err)
	}

	status, err := store.GetOnboardingStatus(testutil.TenantCtx(tenantID), tid)
	if err != nil {
		t.Fatalf("GetOnboardingStatus: %v", err)
	}
	if status["account_type"] != "business" {
		t.Errorf("account_type = %v, want business", status["account_type"])
	}
	if status["workspace_configured"] != true {
		t.Errorf("workspace_configured = %v, want true", status["workspace_configured"])
	}
}

func TestUpdateTenantSettings_MultipleKeys(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()
	ctx := testutil.TenantCtx(tenantID)

	settings := map[string]any{
		"account_type": "personal",
		"account_name": "Test User",
		"industry":     "technology",
		"team_size":    "small",
	}
	for k, v := range settings {
		if err := store.UpdateTenantSettings(ctx, tid, k, v); err != nil {
			t.Fatalf("UpdateTenantSettings(%s): %v", k, err)
		}
	}

	status, err := store.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("GetOnboardingStatus: %v", err)
	}
	if status["account_type"] != "personal" {
		t.Errorf("account_type = %v, want personal", status["account_type"])
	}
	if status["industry"] != "technology" {
		t.Errorf("industry = %v, want technology", status["industry"])
	}
	if status["team_size"] != "small" {
		t.Errorf("team_size = %v, want small", status["team_size"])
	}
}

// --- UpdateTenantBranding ---

func TestUpdateTenantBranding_BothFields(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()
	ctx := testutil.TenantCtx(tenantID)

	if err := store.UpdateTenantBranding(ctx, tid, "#3B82F6", "MyProduct"); err != nil {
		t.Fatalf("UpdateTenantBranding: %v", err)
	}

	status, err := store.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("GetOnboardingStatus: %v", err)
	}
	if status["primary_color"] != "#3B82F6" {
		t.Errorf("primary_color = %v, want #3B82F6", status["primary_color"])
	}
	if status["product_name"] != "MyProduct" {
		t.Errorf("product_name = %v, want MyProduct", status["product_name"])
	}
	if status["branding_set"] != true {
		t.Errorf("branding_set = %v, want true", status["branding_set"])
	}
}

func TestUpdateTenantBranding_PartialUpdate_ColorOnly(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()
	ctx := testutil.TenantCtx(tenantID)

	// Set both first
	if err := store.UpdateTenantBranding(ctx, tid, "#FF0000", "Original"); err != nil {
		t.Fatalf("initial branding: %v", err)
	}

	// Update only color — product_name should stay
	if err := store.UpdateTenantBranding(ctx, tid, "#00FF00", ""); err != nil {
		t.Fatalf("partial update: %v", err)
	}

	status, err := store.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("GetOnboardingStatus: %v", err)
	}
	if status["primary_color"] != "#00FF00" {
		t.Errorf("primary_color = %v, want #00FF00", status["primary_color"])
	}
	if status["product_name"] != "Original" {
		t.Errorf("product_name = %v, want Original (preserved)", status["product_name"])
	}
}

func TestUpdateTenantBranding_PartialUpdate_NameOnly(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()
	ctx := testutil.TenantCtx(tenantID)

	if err := store.UpdateTenantBranding(ctx, tid, "#AABBCC", "First"); err != nil {
		t.Fatalf("initial: %v", err)
	}
	if err := store.UpdateTenantBranding(ctx, tid, "", "Second"); err != nil {
		t.Fatalf("partial: %v", err)
	}

	status, err := store.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("GetOnboardingStatus: %v", err)
	}
	if status["primary_color"] != "#AABBCC" {
		t.Errorf("primary_color = %v, want #AABBCC (preserved)", status["primary_color"])
	}
	if status["product_name"] != "Second" {
		t.Errorf("product_name = %v, want Second", status["product_name"])
	}
}

// --- GetOnboardingStatus ---

func TestGetOnboardingStatus_Empty(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()

	status, err := store.GetOnboardingStatus(testutil.TenantCtx(tenantID), tid)
	if err != nil {
		t.Fatalf("GetOnboardingStatus: %v", err)
	}
	if status["onboarding_complete"] != false {
		t.Errorf("onboarding_complete = %v, want false", status["onboarding_complete"])
	}
	if status["workspace_configured"] != false {
		t.Errorf("workspace_configured = %v, want false", status["workspace_configured"])
	}
	if status["branding_set"] != false {
		t.Errorf("branding_set = %v, want false", status["branding_set"])
	}
}

func TestGetOnboardingStatus_NonExistentTenant(t *testing.T) {
	store, _ := setupOnboardingTest(t)
	fakeID := uuid.Must(uuid.NewV7()).String()

	status, err := store.GetOnboardingStatus(testutil.TenantCtx(uuid.Must(uuid.NewV7())), fakeID)
	if err != nil {
		t.Fatalf("GetOnboardingStatus should not error for missing tenant: %v", err)
	}
	if status["onboarding_complete"] != false {
		t.Errorf("expected false for missing tenant")
	}
}

// --- CompleteOnboarding ---

func TestCompleteOnboarding(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()
	ctx := testutil.TenantCtx(tenantID)

	if err := store.CompleteOnboarding(ctx, tid); err != nil {
		t.Fatalf("CompleteOnboarding: %v", err)
	}

	status, err := store.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("GetOnboardingStatus: %v", err)
	}
	if status["onboarding_complete"] != true {
		t.Errorf("onboarding_complete = %v, want true", status["onboarding_complete"])
	}
	if _, ok := status["completed_at"]; !ok {
		t.Error("completed_at should be set")
	}
}

func TestCompleteOnboarding_Idempotent(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()
	ctx := testutil.TenantCtx(tenantID)

	if err := store.CompleteOnboarding(ctx, tid); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := store.CompleteOnboarding(ctx, tid); err != nil {
		t.Fatalf("second call should not error: %v", err)
	}

	status, err := store.GetOnboardingStatus(ctx, tid)
	if err != nil {
		t.Fatalf("GetOnboardingStatus: %v", err)
	}
	if status["onboarding_complete"] != true {
		t.Errorf("onboarding_complete = %v, want true after idempotent calls", status["onboarding_complete"])
	}
}

// --- PBT: account_type round-trip ---

func TestPBT_AccountType_RoundTrip(t *testing.T) {
	store, tenantID := setupOnboardingTest(t)
	tid := tenantID.String()
	ctx := testutil.TenantCtx(tenantID)

	f := func(val uint8) bool {
		// Use valid account types only
		types := []string{"personal", "business"}
		accType := types[val%2]

		if err := store.UpdateTenantSettings(ctx, tid, "account_type", accType); err != nil {
			return false
		}
		status, err := store.GetOnboardingStatus(ctx, tid)
		if err != nil {
			return false
		}
		return status["account_type"] == accType
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Errorf("PBT round-trip failed: %v", err)
	}
}

// --- Tenant Isolation ---

func TestOnboardingStore_TenantIsolation(t *testing.T) {
	db := testutil.SetupDB(t)
	s := pg.NewPGOnboardingStore(db)

	slugA := "iso-a-" + uuid.Must(uuid.NewV7()).String()[:8]
	slugB := "iso-b-" + uuid.Must(uuid.NewV7()).String()[:8]
	tenantA := testutil.CreateTestTenant(t, db, slugA, "Tenant A")
	tenantB := testutil.CreateTestTenant(t, db, slugB, "Tenant B")
	t.Cleanup(func() {
		db.ExecContext(testutil.TenantCtx(tenantA), "DELETE FROM setup_progress WHERE tenant_id = $1", tenantA)
		db.ExecContext(testutil.TenantCtx(tenantA), "DELETE FROM tenant_branding WHERE tenant_id = $1", tenantA)
		db.ExecContext(testutil.TenantCtx(tenantB), "DELETE FROM setup_progress WHERE tenant_id = $1", tenantB)
		db.ExecContext(testutil.TenantCtx(tenantB), "DELETE FROM tenant_branding WHERE tenant_id = $1", tenantB)
	})

	ctxA := testutil.TenantCtx(tenantA)
	ctxB := testutil.TenantCtx(tenantB)
	tidA := tenantA.String()
	tidB := tenantB.String()

	// Configure tenant A
	if err := s.UpdateTenantSettings(ctxA, tidA, "account_type", "business"); err != nil {
		t.Fatalf("A settings: %v", err)
	}
	if err := s.UpdateTenantBranding(ctxA, tidA, "#FF0000", "TenantA Product"); err != nil {
		t.Fatalf("A branding: %v", err)
	}

	// Tenant B should have empty status
	statusB, err := s.GetOnboardingStatus(ctxB, tidB)
	if err != nil {
		t.Fatalf("B status: %v", err)
	}
	if statusB["workspace_configured"] != false {
		t.Errorf("tenant B should not see tenant A's workspace config")
	}
	if statusB["branding_set"] != false {
		t.Errorf("tenant B should not see tenant A's branding")
	}
	if _, ok := statusB["account_type"]; ok {
		t.Errorf("tenant B should not have account_type from tenant A")
	}
}
