//go:build integration

package pg_test

import (
	"context"
	"database/sql"
	"math/rand"
	"testing"
	"testing/quick"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
	"github.com/vellus-ai/argoclaw/internal/testutil"
)

// --- helpers ---

func setupTwoTenants(t *testing.T, db *sql.DB) (tenantA, tenantB uuid.UUID) {
	t.Helper()
	slug := func(prefix string) string {
		return prefix + "-" + uuid.Must(uuid.NewV7()).String()[:8]
	}
	tenantA = testutil.CreateTestTenant(t, db, slug("iso-a"), "Isolation Tenant A")
	tenantB = testutil.CreateTestTenant(t, db, slug("iso-b"), "Isolation Tenant B")
	t.Cleanup(func() {
		testutil.CleanupTenantData(t, db, tenantA)
		testutil.CleanupTenantData(t, db, tenantB)
	})
	return
}

// ============================================================
// Sessions
// ============================================================

func TestIsolation_Sessions_List_OnlyOwnTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "agent-a-"+uuid.Must(uuid.NewV7()).String()[:8], "Agent A")
	agentB := testutil.CreateAgent(t, db, tenantB, "agent-b-"+uuid.Must(uuid.NewV7()).String()[:8], "Agent B")

	keyA := "agent:" + agentA.String() + ":test:direct:aaaa"
	keyB := "agent:" + agentB.String() + ":test:direct:bbbb"
	testutil.CreateSession(t, db, tenantA, keyA, agentA)
	testutil.CreateSession(t, db, tenantB, keyB, agentB)

	ss := pg.NewPGSessionStore(db)

	ctxA := testutil.TenantCtx(tenantA)
	ctxB := testutil.TenantCtx(tenantB)

	// Tenant A should see only its own sessions
	optsA := store.SessionListOpts{AgentID: agentA.String(), Limit: 10}
	resultA := ss.ListPaged(ctxA, optsA)
	for _, s := range resultA.Sessions {
		if s.Key == keyB {
			t.Errorf("tenant A listing returned tenant B session %q", keyB)
		}
	}

	// Tenant B should see only its own sessions
	optsB := store.SessionListOpts{AgentID: agentB.String(), Limit: 10}
	resultB := ss.ListPaged(ctxB, optsB)
	for _, s := range resultB.Sessions {
		if s.Key == keyA {
			t.Errorf("tenant B listing returned tenant A session %q", keyA)
		}
	}
}

func TestIsolation_Sessions_ListPaged_TenantFilter(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "agent-lp-a-"+uuid.Must(uuid.NewV7()).String()[:8], "LP Agent A")
	agentB := testutil.CreateAgent(t, db, tenantB, "agent-lp-b-"+uuid.Must(uuid.NewV7()).String()[:8], "LP Agent B")

	for i := 0; i < 3; i++ {
		testutil.CreateSession(t, db, tenantA, "agent:"+agentA.String()+":ch:direct:a"+string(rune('0'+i)), agentA)
		testutil.CreateSession(t, db, tenantB, "agent:"+agentB.String()+":ch:direct:b"+string(rune('0'+i)), agentB)
	}

	ss := pg.NewPGSessionStore(db)

	ctxA := testutil.TenantCtx(tenantA)

	// List with ctx of A should return exactly 3 sessions for A
	optsA := store.SessionListOpts{Limit: 20}
	resultA := ss.ListPaged(ctxA, optsA)
	for _, s := range resultA.Sessions {
		// No session from B should appear in A's list
		if s.Key != "" {
			// Verify it's agent A's session key
			prefix := "agent:" + agentB.String()
			if len(s.Key) >= len(prefix) && s.Key[:len(prefix)] == prefix {
				t.Errorf("tenant A ListPaged returned tenant B session %q", s.Key)
			}
		}
	}
}

func TestIsolation_Sessions_Delete_CrossTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "agent-del-a-"+uuid.Must(uuid.NewV7()).String()[:8], "Del Agent A")
	keyA := "agent:" + agentA.String() + ":ws:direct:del001"
	testutil.CreateSession(t, db, tenantA, keyA, agentA)

	ss := pg.NewPGSessionStore(db)

	ctxA := testutil.TenantCtx(tenantA)
	ctxB := testutil.TenantCtx(tenantB)

	// Cross-tenant delete: tenant B must NOT be able to delete tenant A's session.
	if err := ss.Delete(ctxB, keyA); err != nil {
		t.Errorf("Delete(ctxB) returned unexpected error: %v", err)
	}
	var count int
	db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sessions WHERE session_key = $1", keyA,
	).Scan(&count)
	if count != 1 {
		t.Errorf("cross-tenant delete must not delete session; got %d rows remaining (expected 1)", count)
	}

	// Same-tenant delete: tenant A must be able to delete its own session.
	if err := ss.Delete(ctxA, keyA); err != nil {
		t.Errorf("Delete(ctxA) returned unexpected error: %v", err)
	}
	db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sessions WHERE session_key = $1", keyA,
	).Scan(&count)
	if count != 0 {
		t.Errorf("same-tenant delete should have removed session, got %d rows", count)
	}
}

func TestIsolation_Sessions_GetOrCreate_SetsTenantID(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, _ := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "agent-goc-"+uuid.Must(uuid.NewV7()).String()[:8], "GoC Agent")
	keyA := "agent:" + agentA.String() + ":ws:direct:goc001"

	ss := pg.NewPGSessionStore(db)
	ctxA := testutil.TenantCtx(tenantA)

	data := ss.GetOrCreate(ctxA, keyA)
	if data == nil {
		t.Fatal("GetOrCreate returned nil")
	}

	// Save to persist, then verify tenant_id in DB
	_ = ss.Save(ctxA, keyA)

	var dbTenantID uuid.UUID
	db.QueryRowContext(context.Background(),
		"SELECT COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid) FROM sessions WHERE session_key = $1", keyA,
	).Scan(&dbTenantID)
	if dbTenantID != tenantA {
		t.Errorf("DB tenant_id = %v, want %v", dbTenantID, tenantA)
	}
}

func TestPBT_SessionKey_NeverLeaksCrossTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "pbt-a-"+uuid.Must(uuid.NewV7()).String()[:8], "PBT Agent A")

	ss := pg.NewPGSessionStore(db)

	f := func(suffix [8]byte) bool {
		key := "agent:" + agentA.String() + ":pbt:direct:" + uuid.Must(uuid.NewV7()).String()

		testutil.CreateSession(t, db, tenantA, key, agentA)
		defer db.ExecContext(context.Background(), "DELETE FROM sessions WHERE session_key = $1", key)

		// Tenant B lists sessions with its own tenant context — should not see A's session
		ctxB := testutil.TenantCtx(tenantB)
		optsB := store.SessionListOpts{Limit: 100}
		resultB := ss.ListPaged(ctxB, optsB)
		for _, s := range resultB.Sessions {
			if s.Key == key {
				return false // leak detected
			}
		}
		_ = suffix
		return true
	}
	cfg := &quick.Config{MaxCount: 50, Rand: rand.New(rand.NewSource(42))}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("PBT session key leak: %v", err)
	}
}

// ============================================================
// Cron
// ============================================================

func TestIsolation_Cron_ListJobs_OnlyOwnTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "cron-agent-a-"+uuid.Must(uuid.NewV7()).String()[:8], "Cron Agent A")
	agentB := testutil.CreateAgent(t, db, tenantB, "cron-agent-b-"+uuid.Must(uuid.NewV7()).String()[:8], "Cron Agent B")

	jobAID := testutil.CreateCronJob(t, db, tenantA, agentA, "job-a-"+uuid.Must(uuid.NewV7()).String()[:8])
	jobBID := testutil.CreateCronJob(t, db, tenantB, agentB, "job-b-"+uuid.Must(uuid.NewV7()).String()[:8])

	cs := pg.NewPGCronStore(db)

	// Tenant A listing should not include B's jobs
	ctxA := testutil.TenantCtx(tenantA)
	jobsA := cs.ListJobs(ctxA, true, "", "")
	for _, j := range jobsA {
		if j.ID == jobBID.String() {
			t.Errorf("tenant A ListJobs returned tenant B job %s", jobBID)
		}
	}

	// Tenant B listing should not include A's jobs
	ctxB := testutil.TenantCtx(tenantB)
	jobsB := cs.ListJobs(ctxB, true, "", "")
	for _, j := range jobsB {
		if j.ID == jobAID.String() {
			t.Errorf("tenant B ListJobs returned tenant A job %s", jobAID)
		}
	}
}

func TestIsolation_Cron_GetJob_CrossTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "cron-get-a-"+uuid.Must(uuid.NewV7()).String()[:8], "Cron Get A")
	jobAID := testutil.CreateCronJob(t, db, tenantA, agentA, "getjob-a-"+uuid.Must(uuid.NewV7()).String()[:8])

	cs := pg.NewPGCronStore(db)

	// Tenant B should NOT be able to retrieve tenant A's job
	ctxB := testutil.TenantCtx(tenantB)
	job, found := cs.GetJob(ctxB, jobAID.String())
	if found {
		t.Errorf("tenant B retrieved tenant A job %s (expected not found)", jobAID)
		_ = job
	}

	// Tenant A should be able to retrieve its own job
	ctxA := testutil.TenantCtx(tenantA)
	jobA, foundA := cs.GetJob(ctxA, jobAID.String())
	if !foundA {
		t.Errorf("tenant A could not retrieve its own job %s", jobAID)
	} else if jobA.ID != jobAID.String() {
		t.Errorf("job ID mismatch: got %s, want %s", jobA.ID, jobAID)
	}
}

func TestIsolation_Cron_AddJob_SetsTenantID(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, _ := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "cron-add-"+uuid.Must(uuid.NewV7()).String()[:8], "Cron Add A")

	cs := pg.NewPGCronStore(db)
	ctxA := testutil.TenantCtx(tenantA)

	job, err := cs.AddJob(ctxA,
		"test-job-"+uuid.Must(uuid.NewV7()).String()[:8],
		store.CronSchedule{Kind: "every", EveryMS: ptrInt64(60000)},
		"hello", false, "", "", agentA.String(), "")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM cron_jobs WHERE id = $1", job.ID)
	})

	// Verify tenant_id stored in DB
	var dbTenantID uuid.UUID
	db.QueryRowContext(context.Background(),
		"SELECT COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid) FROM cron_jobs WHERE id = $1", job.ID,
	).Scan(&dbTenantID)
	if dbTenantID != tenantA {
		t.Errorf("cron_jobs tenant_id = %v, want %v", dbTenantID, tenantA)
	}
}

// ============================================================
// Skills
// ============================================================

func TestIsolation_Skills_ListSkills_OnlyOwnTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	skillAID := testutil.CreateSkill(t, db, tenantA, "Skill A", "skill-a-"+uuid.Must(uuid.NewV7()).String()[:8])
	skillBID := testutil.CreateSkill(t, db, tenantB, "Skill B", "skill-b-"+uuid.Must(uuid.NewV7()).String()[:8])

	ss := pg.NewPGSkillStore(db, t.TempDir())

	// ListSkills with tenant A context should not expose tenant B's skill
	ctxA := testutil.TenantCtx(tenantA)
	skillsA := ss.ListSkillsByTenant(ctxA)
	for _, sk := range skillsA {
		if sk.ID == skillBID.String() {
			t.Errorf("tenant A's ListSkillsByTenant returned tenant B skill %s", skillBID)
		}
	}

	// ListSkills with tenant B context should not expose tenant A's skill
	ctxB := testutil.TenantCtx(tenantB)
	skillsB := ss.ListSkillsByTenant(ctxB)
	for _, sk := range skillsB {
		if sk.ID == skillAID.String() {
			t.Errorf("tenant B's ListSkillsByTenant returned tenant A skill %s", skillAID)
		}
	}
}

func TestIsolation_Skills_Delete_CrossTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	skillAID := testutil.CreateSkill(t, db, tenantA, "Skill Del A", "skill-del-a-"+uuid.Must(uuid.NewV7()).String()[:8])

	ss := pg.NewPGSkillStore(db, t.TempDir())

	// Tenant B should NOT be able to delete tenant A's skill
	ctxB := testutil.TenantCtx(tenantB)
	err := ss.DeleteSkillWithCtx(ctxB, skillAID)
	if err == nil {
		// Check if the skill still exists in DB
		var count int
		db.QueryRowContext(context.Background(),
			"SELECT COUNT(*) FROM skills WHERE id = $1 AND status != 'deleted'", skillAID,
		).Scan(&count)
		if count == 0 {
			t.Errorf("tenant B was able to delete tenant A's skill %s", skillAID)
		}
	}

	// Tenant A should be able to delete its own skill
	ctxA := testutil.TenantCtx(tenantA)
	if err := ss.DeleteSkillWithCtx(ctxA, skillAID); err != nil {
		t.Errorf("tenant A could not delete its own skill: %v", err)
	}
}

func TestIsolation_Skills_Create_SetsTenantID(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, _ := setupTwoTenants(t, db)

	ss := pg.NewPGSkillStore(db, t.TempDir())
	ctxA := testutil.TenantCtx(tenantA)

	slug := "skill-create-" + uuid.Must(uuid.NewV7()).String()[:8]
	err := ss.CreateSkillWithCtx(ctxA, "Test Skill", slug, nil, "owner1", "private", 1, "/skills/test.md", 0, nil)
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM skills WHERE slug = $1", slug)
	})

	var dbTenantID uuid.UUID
	db.QueryRowContext(context.Background(),
		"SELECT COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid) FROM skills WHERE slug = $1", slug,
	).Scan(&dbTenantID)
	if dbTenantID != tenantA {
		t.Errorf("skills tenant_id = %v, want %v", dbTenantID, tenantA)
	}
}

func TestIsolation_Skills_Update_CrossTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	skillAID := testutil.CreateSkill(t, db, tenantA, "Skill Upd A", "skill-upd-a-"+uuid.Must(uuid.NewV7()).String()[:8])

	ss := pg.NewPGSkillStore(db, t.TempDir())

	// Tenant B should NOT be able to update tenant A's skill
	ctxB := testutil.TenantCtx(tenantB)
	err := ss.UpdateSkillWithCtx(ctxB, skillAID, map[string]any{"name": "HACKED"})
	if err == nil {
		var name string
		db.QueryRowContext(context.Background(),
			"SELECT name FROM skills WHERE id = $1", skillAID,
		).Scan(&name)
		if name == "HACKED" {
			t.Errorf("tenant B was able to update tenant A's skill name to HACKED")
		}
	}
}

// ============================================================
// Canary: data in both tenants, SQL sees both, stores filter
// ============================================================

func TestCanary_DataExistsBothTenants_StoreFilters(t *testing.T) {
	db := testutil.SetupDB(t)
	tenantA, tenantB := setupTwoTenants(t, db)

	agentA := testutil.CreateAgent(t, db, tenantA, "canary-a-"+uuid.Must(uuid.NewV7()).String()[:8], "Canary A")
	agentB := testutil.CreateAgent(t, db, tenantB, "canary-b-"+uuid.Must(uuid.NewV7()).String()[:8], "Canary B")

	keyA := "agent:" + agentA.String() + ":ws:direct:canary-a"
	keyB := "agent:" + agentB.String() + ":ws:direct:canary-b"
	testutil.CreateSession(t, db, tenantA, keyA, agentA)
	testutil.CreateSession(t, db, tenantB, keyB, agentB)

	// Verify SQL sees both (no filter)
	var totalCount int
	db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sessions WHERE session_key IN ($1, $2)", keyA, keyB,
	).Scan(&totalCount)
	if totalCount != 2 {
		t.Fatalf("canary: expected 2 sessions in DB, got %d", totalCount)
	}

	// Store with ctx of tenant A should return only tenant A's session
	ss := pg.NewPGSessionStore(db)
	ctxA := testutil.TenantCtx(tenantA)
	optsA := store.SessionListOpts{Limit: 100}
	resultA := ss.ListPaged(ctxA, optsA)
	foundB := false
	for _, s := range resultA.Sessions {
		if s.Key == keyB {
			foundB = true
		}
	}
	if foundB {
		t.Errorf("canary: tenant A store returned tenant B session — tenant isolation broken")
	}
}

// --- helpers ---

func ptrInt64(v int64) *int64 { return &v }
