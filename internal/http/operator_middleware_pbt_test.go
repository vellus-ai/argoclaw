package http

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 4.1 — requireOperatorRole PBT
// **Validates: Requirements 3.8, 3.9, 3.10, 9.1, 9.2, 9.3, 9.4, 9.8**
//
// Property 1 (Central System Invariant — Biconditional):
//   ∀ (operator_level OL, role R):
//     requireOperatorRole allows access ↔ (OL >= 1 AND R >= RoleOperator)
//
// Property 2 (Security Logging):
//   ∀ rejection: log "security.operator_access_denied" is emitted
// ─────────────────────────────────────────────────────────────────────────────

// jwtRole maps operator_level-style role strings to JWT claim role strings.
// The middleware uses resolveAuth → JWT claims → JWTRoleToPermission.
var jwtRoleMap = map[string]string{
	"viewer":   "viewer",
	"operator": "member", // "member" maps to RoleOperator via JWTRoleToPermission
	"admin":    "admin",
}

// buildContext constructs a request context with the given operator_level and role.
// operator_level >= 1 → WithOperatorMode + WithCrossTenant; 0 → normal tenant context.
func buildContext(operatorLevel int, role string) context.Context {
	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "pbt-user")

	if operatorLevel >= 1 {
		ctx = store.WithCrossTenant(ctx)        // appsec:cross-tenant-bypass — PBT test
		ctx = store.WithOperatorMode(ctx, tenantID)
	}

	jwtRole := jwtRoleMap[role]
	ctx = injectClaims(ctx, jwtRole)
	return ctx
}

// shouldAllow returns true iff the access matrix permits the combination.
// Central invariant: access ↔ (operator_level >= 1 AND role >= RoleOperator).
func shouldAllow(operatorLevel int, role string) bool {
	if operatorLevel < 1 {
		return false
	}
	switch role {
	case "operator", "admin":
		return true
	default:
		return false
	}
}

// TestRequireOperatorRole_PBT_Biconditional verifies the central system invariant:
//
//	∀ (OL ∈ {0, 1, 2}, R ∈ {viewer, operator, admin}):
//	  requireOperatorRole allows access ↔ (OL >= 1 AND R >= RoleOperator)
func TestRequireOperatorRole_PBT_Biconditional(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		operatorLevel := rapid.IntRange(0, 2).Draw(t, "operator_level")
		role := rapid.SampledFrom([]string{"viewer", "operator", "admin"}).Draw(t, "role")

		ctx := buildContext(operatorLevel, role)

		called := false
		handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler(rec, req)

		expected := shouldAllow(operatorLevel, role)

		if expected && !called {
			t.Fatalf("OL=%d role=%s: handler not called, expected access allowed (status=%d body=%s)",
				operatorLevel, role, rec.Code, rec.Body.String())
		}
		if !expected && called {
			t.Fatalf("OL=%d role=%s: handler was called, expected access denied",
				operatorLevel, role)
		}
		if expected && rec.Code != http.StatusOK {
			t.Fatalf("OL=%d role=%s: status=%d, want 200", operatorLevel, role, rec.Code)
		}
		if !expected && rec.Code != http.StatusForbidden {
			t.Fatalf("OL=%d role=%s: status=%d, want 403", operatorLevel, role, rec.Code)
		}

		// Verify correct error code on rejection
		if !expected {
			body := rec.Body.String()
			if operatorLevel < 1 {
				if !strings.Contains(body, ErrCodeOperatorRequired) {
					t.Fatalf("OL=%d role=%s: body=%s, want %s", operatorLevel, role, body, ErrCodeOperatorRequired)
				}
			} else {
				if !strings.Contains(body, ErrCodeInsufficientRole) {
					t.Fatalf("OL=%d role=%s: body=%s, want %s", operatorLevel, role, body, ErrCodeInsufficientRole)
				}
			}
		}
	})
}

// TestRequireOperatorRole_PBT_SecurityLogging verifies that every rejection
// emits a "security.operator_access_denied" log entry.
func TestRequireOperatorRole_PBT_SecurityLogging(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		operatorLevel := rapid.IntRange(0, 2).Draw(t, "operator_level")
		role := rapid.SampledFrom([]string{"viewer", "operator", "admin"}).Draw(t, "role")

		if shouldAllow(operatorLevel, role) {
			return // skip allowed combinations — no rejection log expected
		}

		// Capture slog output
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		orig := slog.Default()
		slog.SetDefault(logger)
		defer slog.SetDefault(orig)

		ctx := buildContext(operatorLevel, role)

		handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler(rec, req)

		logOutput := buf.String()
		if !strings.Contains(logOutput, "security.operator_access_denied") {
			t.Fatalf("OL=%d role=%s: expected security.operator_access_denied log, got: %s",
				operatorLevel, role, logOutput)
		}

		// Verify the reason field matches the error code
		if operatorLevel < 1 {
			if !strings.Contains(logOutput, ErrCodeOperatorRequired) {
				t.Fatalf("OL=%d role=%s: log missing reason=%s, got: %s",
					operatorLevel, role, ErrCodeOperatorRequired, logOutput)
			}
		} else {
			if !strings.Contains(logOutput, ErrCodeInsufficientRole) {
				t.Fatalf("OL=%d role=%s: log missing reason=%s, got: %s",
					operatorLevel, role, ErrCodeInsufficientRole, logOutput)
			}
		}
	})
}
