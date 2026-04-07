package plugins

import (
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 9.6 — CircuitBreaker unit tests (TDD RED)
// Validates: Requirement 24.2
// ─────────────────────────────────────────────────────────────────────────────

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker("test-plugin", 5, 30*time.Second)

	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state %q, got %q", CircuitClosed, cb.State())
	}
	if !cb.Allow() {
		t.Error("expected Allow() = true for new circuit breaker")
	}
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	tests := []struct {
		name      string
		threshold int
		failures  int
		wantState CircuitState
		wantAllow bool
	}{
		{
			name:      "below threshold stays closed",
			threshold: 5,
			failures:  4,
			wantState: CircuitClosed,
			wantAllow: true,
		},
		{
			name:      "at threshold opens circuit",
			threshold: 5,
			failures:  5,
			wantState: CircuitOpen,
			wantAllow: false,
		},
		{
			name:      "above threshold stays open",
			threshold: 5,
			failures:  7,
			wantState: CircuitOpen,
			wantAllow: false,
		},
		{
			name:      "threshold of 1 opens on first failure",
			threshold: 1,
			failures:  1,
			wantState: CircuitOpen,
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker("test", tt.threshold, 30*time.Second)
			for i := 0; i < tt.failures; i++ {
				cb.RecordFailure()
			}
			if got := cb.State(); got != tt.wantState {
				t.Errorf("State() = %q, want %q", got, tt.wantState)
			}
			if got := cb.Allow(); got != tt.wantAllow {
				t.Errorf("Allow() = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	resetTimeout := 50 * time.Millisecond
	cb := NewCircuitBreaker("test", 2, resetTimeout)

	// Drive to open state.
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("expected open, got %q", cb.State())
	}
	if cb.Allow() {
		t.Fatal("expected Allow() = false while open")
	}

	// Wait for reset timeout to elapse.
	time.Sleep(resetTimeout + 10*time.Millisecond)

	// After timeout, Allow() should return true (half-open probe).
	if !cb.Allow() {
		t.Error("expected Allow() = true after resetTimeout (half-open)")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected state %q, got %q", CircuitHalfOpen, cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	resetTimeout := 50 * time.Millisecond
	cb := NewCircuitBreaker("test", 2, resetTimeout)

	// Drive to open → half-open.
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(resetTimeout + 10*time.Millisecond)
	cb.Allow() // transitions to half-open

	// Record success → should close.
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Errorf("expected state %q after success in half-open, got %q", CircuitClosed, cb.State())
	}
	if !cb.Allow() {
		t.Error("expected Allow() = true after closing circuit")
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	resetTimeout := 50 * time.Millisecond
	cb := NewCircuitBreaker("test", 2, resetTimeout)

	// Drive to open → half-open.
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(resetTimeout + 10*time.Millisecond)
	cb.Allow() // transitions to half-open

	// Record failure in half-open → should re-open.
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("expected state %q after failure in half-open, got %q", CircuitOpen, cb.State())
	}
	if cb.Allow() {
		t.Error("expected Allow() = false after re-opening circuit")
	}
}

func TestCircuitBreaker_AllowReturnsFalseWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 1*time.Hour) // long timeout so it stays open

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Multiple calls to Allow() should all return false while open.
	for i := 0; i < 10; i++ {
		if cb.Allow() {
			t.Fatalf("Allow() returned true on call %d while circuit is open", i)
		}
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 30*time.Second)

	// Record 4 failures (below threshold).
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	// Record a success — should reset failure count.
	cb.RecordSuccess()

	// Now 4 more failures should NOT open the circuit (count was reset).
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after reset + 4 failures, got %q", cb.State())
	}
	if !cb.Allow() {
		t.Error("expected Allow() = true")
	}
}

func TestCircuitBreaker_Configurability(t *testing.T) {
	tests := []struct {
		name         string
		threshold    int
		resetTimeout time.Duration
	}{
		{"default values", 5, 30 * time.Second},
		{"low threshold", 1, 100 * time.Millisecond},
		{"high threshold", 100, 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker("test", tt.threshold, tt.resetTimeout)

			// Verify threshold: exactly threshold failures should open.
			for i := 0; i < tt.threshold-1; i++ {
				cb.RecordFailure()
			}
			if cb.State() != CircuitClosed {
				t.Errorf("expected closed after %d failures, got %q", tt.threshold-1, cb.State())
			}
			cb.RecordFailure() // the threshold-th failure
			if cb.State() != CircuitOpen {
				t.Errorf("expected open after %d failures, got %q", tt.threshold, cb.State())
			}
		})
	}
}

func TestCircuitBreaker_FullCycle(t *testing.T) {
	resetTimeout := 50 * time.Millisecond
	cb := NewCircuitBreaker("test", 2, resetTimeout)

	// 1. Start closed.
	if cb.State() != CircuitClosed {
		t.Fatalf("step 1: expected closed, got %q", cb.State())
	}

	// 2. Two failures → open.
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("step 2: expected open, got %q", cb.State())
	}

	// 3. Wait → half-open.
	time.Sleep(resetTimeout + 10*time.Millisecond)
	if !cb.Allow() {
		t.Fatal("step 3: expected Allow() = true (half-open)")
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("step 3: expected half-open, got %q", cb.State())
	}

	// 4. Failure in half-open → back to open.
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("step 4: expected open, got %q", cb.State())
	}

	// 5. Wait again → half-open.
	time.Sleep(resetTimeout + 10*time.Millisecond)
	cb.Allow()
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("step 5: expected half-open, got %q", cb.State())
	}

	// 6. Success in half-open → closed.
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Fatalf("step 6: expected closed, got %q", cb.State())
	}

	// 7. Allow works again.
	if !cb.Allow() {
		t.Fatal("step 7: expected Allow() = true after closing")
	}
}
