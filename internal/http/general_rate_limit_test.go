package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeneralRateLimiter_AllowsNormalTraffic(t *testing.T) {
	t.Parallel()

	gl := NewGeneralRateLimiter(60, 10)
	handler := gl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGeneralRateLimiter_BlocksExcessiveRequests(t *testing.T) {
	t.Parallel()

	// 1 rpm, burst 1 — second request should be blocked
	gl := NewGeneralRateLimiter(1, 1)
	handler := gl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "10.0.0.1:9999"

	// First request: allowed (uses the burst token)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = ip
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec.Code)
	}

	// Second request: should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
	req2.RemoteAddr = ip
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") != "60" {
		t.Errorf("expected Retry-After: 60, got %q", rec2.Header().Get("Retry-After"))
	}
}

func TestGeneralRateLimiter_DifferentIPsIndependent(t *testing.T) {
	t.Parallel()

	gl := NewGeneralRateLimiter(1, 1)
	handler := gl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP A: exhaust its budget
	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.RemoteAddr = "10.0.0.1:1111"
	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("IP A first: expected 200, got %d", recA.Code)
	}

	// IP B: should still be allowed
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.RemoteAddr = "10.0.0.2:2222"
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Errorf("IP B first: expected 200, got %d", recB.Code)
	}
}

func TestGeneralRateLimiter_ZeroRPMAllowsAll(t *testing.T) {
	t.Parallel()

	gl := NewGeneralRateLimiter(0, 0)
	handler := gl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
}
