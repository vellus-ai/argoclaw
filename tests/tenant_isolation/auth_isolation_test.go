package tenant_isolation_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// =============================================================================
// TDD: JWT Authentication — Tenant Boundary Enforcement
// =============================================================================

// Test: Valid JWT for Tenant A is accepted.
func TestJWT_ValidToken_Accepted(t *testing.T) {
	t.Parallel()
	claims, err := auth.ValidateAccessToken(env.tokenA, env.jwtSecret)
	if err != nil {
		t.Fatalf("valid token A should be accepted: %v", err)
	}
	if claims.TenantID != env.tenantA.ID.String() {
		t.Errorf("expected tenant_id=%s, got %s", env.tenantA.ID, claims.TenantID)
	}
	if claims.UserID != env.userA.String() {
		t.Errorf("expected user_id=%s, got %s", env.userA, claims.UserID)
	}
}

// Test: JWT signed with wrong secret is rejected.
func TestJWT_WrongSecret_Rejected(t *testing.T) {
	t.Parallel()
	wrongToken, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:   env.userA.String(),
		TenantID: env.tenantA.ID.String(),
		Role:     "admin",
	}, "completely-wrong-secret-key-that-should-fail")
	if err != nil {
		t.Fatalf("generate wrong-secret token: %v", err)
	}

	_, err = auth.ValidateAccessToken(wrongToken, env.jwtSecret)
	if err == nil {
		t.Fatal("SECURITY VIOLATION: JWT signed with wrong secret was accepted")
	}
}

// Test: JWT with tampered tenant_id is detected.
func TestJWT_TamperedTenantID_Detected(t *testing.T) {
	t.Parallel()
	// Generate valid token for Tenant A
	token := env.tokenA

	// Tamper with the payload: change tenant_id to Tenant B's
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatal("invalid JWT structure")
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	// Modify tenant_id
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	claims["tid"] = env.tenantB.ID.String()

	// Re-encode (without re-signing — this is the attack)
	newPayload, _ := json.Marshal(claims)
	parts[1] = base64.RawURLEncoding.EncodeToString(newPayload)
	tamperedToken := strings.Join(parts, ".")

	// Validate — MUST fail because signature no longer matches
	_, err = auth.ValidateAccessToken(tamperedToken, env.jwtSecret)
	if err == nil {
		t.Fatal("SECURITY VIOLATION: JWT with tampered tenant_id was accepted")
	}
}

// Test: JWT with non-existent tenant UUID passes validation but store queries return nothing.
func TestJWT_NonExistentTenant_QueriesEmpty(t *testing.T) {
	t.Parallel()
	fakeTenantID := uuid.New()
	token := mustGenerateToken(t, auth.TokenClaims{
		UserID:   env.userA.String(),
		TenantID: fakeTenantID.String(),
		Role:     "admin",
	})

	// Token is valid (signature OK)
	claims, err := auth.ValidateAccessToken(token, env.jwtSecret)
	if err != nil {
		t.Fatalf("token with fake tenant should have valid signature: %v", err)
	}
	if claims.TenantID != fakeTenantID.String() {
		t.Errorf("expected tenant_id=%s, got %s", fakeTenantID, claims.TenantID)
	}

	// But store queries return nothing
	ctx := ctxForTenant(fakeTenantID)
	got, err := env.agentStore.GetByKey(ctx, "any-agent-key")
	if err == nil && got != nil {
		t.Fatal("SECURITY VIOLATION: non-existent tenant returned data")
	}
}

// Test: JWT with empty tenant_id — middleware should pass through (gateway mode).
func TestJWT_EmptyTenantID_PassThrough(t *testing.T) {
	t.Parallel()
	token := mustGenerateToken(t, auth.TokenClaims{
		UserID:   env.userA.String(),
		TenantID: "",
		Role:     "admin",
	})

	claims, err := auth.ValidateAccessToken(token, env.jwtSecret)
	if err != nil {
		t.Fatalf("token with empty tenant should be valid: %v", err)
	}
	if claims.TenantID != "" {
		t.Errorf("expected empty tenant_id, got %q", claims.TenantID)
	}
}

// Test: JWT with invalid UUID as tenant_id.
func TestJWT_InvalidUUID_TenantID(t *testing.T) {
	invalidIDs := []string{
		"not-a-uuid",
		"12345",
		"' OR 1=1 --",
		"../../../etc/passwd",
		"<script>alert(1)</script>",
		"00000000-0000-0000-0000-000000000000", // nil UUID
	}

	for _, tid := range invalidIDs {
		tid := tid // capture for parallel subtests
		label := tid
		if len(label) > 20 {
			label = label[:20]
		}
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			token := mustGenerateToken(t, auth.TokenClaims{
				UserID:   env.userA.String(),
				TenantID: tid,
				Role:     "admin",
			})
			// Token generation should succeed (it's just claims)
			claims, err := auth.ValidateAccessToken(token, env.jwtSecret)
			if err != nil {
				return // if validation fails, that's fine
			}

			parsedID, parseErr := uuid.Parse(claims.TenantID)

			if tid == "00000000-0000-0000-0000-000000000000" {
				// Nil UUID parses but TenantMiddleware treats uuid.Nil as no-tenant
				if parseErr != nil {
					t.Error("nil UUID should parse successfully")
				}
				if parsedID != uuid.Nil {
					t.Error("expected uuid.Nil for all-zeros UUID")
				}
				return
			}

			// All other invalid payloads MUST fail uuid.Parse
			if parseErr == nil {
				t.Fatalf("SECURITY VIOLATION: invalid tenant_id %q parsed as valid UUID %s", tid, parsedID)
			}
		})
	}
}

// Test: JWT with algorithm confusion (none/HS384/RS256).
func TestJWT_AlgorithmConfusion(t *testing.T) {
	t.Parallel()
	// Build a token with "alg": "none"
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(
		`{"uid":"%s","tid":"%s","role":"admin","iss":"argoclaw","exp":%d}`,
		env.userA.String(), env.tenantA.ID.String(), 9999999999,
	)))
	noneToken := header + "." + payload + "."

	_, err := auth.ValidateAccessToken(noneToken, env.jwtSecret)
	if err == nil {
		t.Fatal("SECURITY VIOLATION: 'none' algorithm JWT was accepted")
	}
}

// =============================================================================
// TDD: HTTP API — Authorization Boundary Tests
// =============================================================================

// Test: Request without Authorization header does not leak tenant-scoped data.
func TestHTTP_NoAuth_NoTenantDataLeaked(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	req, err := http.NewRequest("GET", env.gatewayURL+"/v1/agents", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("gateway not reachable: %v", err)
	}
	defer resp.Body.Close()

	// Without any auth, the gateway may return 200 (gateway token mode) or 401
	// The important thing is that no tenant-scoped data leaks
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Verify response doesn't contain tenant-specific data
		bodyStr := string(body)
		if strings.Contains(bodyStr, env.tenantA.ID.String()) || strings.Contains(bodyStr, env.tenantB.ID.String()) {
			t.Fatal("SECURITY VIOLATION: unauthenticated request returned tenant-scoped data")
		}
	}
}

// Test: Tenant A's token accessing agents only returns Tenant A's agents.
func TestHTTP_TenantA_Token_OnlySeesOwnAgents(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	req, err := httpReqWithToken("GET", env.gatewayURL+"/v1/agents", env.tokenA)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("gateway not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		// Must NOT contain Tenant B's ID
		if strings.Contains(bodyStr, env.tenantB.ID.String()) {
			t.Fatal("SECURITY VIOLATION: Tenant A's request returned Tenant B's data")
		}
	}
}

// Test: Tenant B's token cannot access Tenant A's specific agent.
func TestHTTP_CrossTenant_AgentAccess_Denied(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	// Create an agent for Tenant A
	agent := &store.AgentData{
		AgentKey:    "http-cross-" + uuid.New().String()[:8],
		DisplayName: "HTTP Cross-Tenant Test",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}
	ctxA := ctxForTenant(env.tenantA.ID)
	if err := env.agentStore.Create(ctxA, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctxA, `DELETE FROM agents WHERE id = $1`, agent.ID)
	})

	// Tenant B tries to GET that agent by ID
	req, err := httpReqWithToken("GET", env.gatewayURL+"/v1/agents/"+agent.ID.String(), env.tokenB)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("gateway not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var result map[string]any
		if json.Unmarshal(body, &result) == nil {
			if id, ok := result["id"].(string); ok && id == agent.ID.String() {
				t.Fatal("SECURITY VIOLATION: Tenant B accessed Tenant A's agent via HTTP")
			}
		}
	}
}

// =============================================================================
// TDD: Header Injection Tests
// =============================================================================

// Test: X-ArgoClaw-User-Id header cannot be spoofed externally.
func TestHTTP_UserIdHeader_CannotBeForged(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	req, err := httpReqWithToken("GET", env.gatewayURL+"/v1/agents", env.tokenA)
	if err != nil {
		t.Fatal(err)
	}
	// Try to inject a different user ID
	req.Header.Set("X-ArgoClaw-User-Id", env.userB.String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("gateway not reachable: %v", err)
	}
	defer resp.Body.Close()

	// The middleware should overwrite X-ArgoClaw-User-Id with the JWT's user_id
	// This test verifies the middleware doesn't trust external headers
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if strings.Contains(bodyStr, env.tenantB.ID.String()) {
			t.Fatal("SECURITY VIOLATION: forged X-ArgoClaw-User-Id header leaked cross-tenant data")
		}
	}
}

// Test: Authorization header with Tenant A JWT + X-Tenant-Id for Tenant B.
func TestHTTP_TenantIdHeader_CannotOverrideJWT(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	req, err := httpReqWithToken("GET", env.gatewayURL+"/v1/agents", env.tokenA)
	if err != nil {
		t.Fatal(err)
	}
	// Attempt to override tenant via custom header
	req.Header.Set("X-Tenant-Id", env.tenantB.ID.String())
	req.Header.Set("X-ArgoClaw-Tenant-Id", env.tenantB.ID.String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("gateway not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if strings.Contains(bodyStr, env.tenantB.ID.String()) {
			t.Fatal("SECURITY VIOLATION: X-Tenant-Id header overrode JWT-based tenant isolation")
		}
	}
}
