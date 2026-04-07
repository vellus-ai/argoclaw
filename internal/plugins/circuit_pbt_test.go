package plugins

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 9.7 — CircuitBreaker property-based test (P8)
// **Validates: Requirements 24.2**
// Property P8: após threshold falhas consecutivas, Allow() retorna false
// ─────────────────────────────────────────────────────────────────────────────

// TestCircuitBreaker_P8_ThresholdInvariant verifies that for any threshold
// and any sequence of operations, the circuit breaker opens (Allow() == false)
// if and only if there have been >= threshold consecutive failures without
// an intervening success.
func TestCircuitBreaker_P8_ThresholdInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		threshold := rapid.IntRange(1, 20).Draw(t, "threshold")
		numOps := rapid.IntRange(1, 100).Draw(t, "numOps")

		cb := NewCircuitBreaker("pbt-plugin", threshold, 1*time.Hour) // long timeout to keep open

		consecutiveFailures := 0

		for i := 0; i < numOps; i++ {
			op := rapid.IntRange(0, 1).Draw(t, "op")

			switch op {
			case 0: // RecordSuccess
				cb.RecordSuccess()
				// Success in closed state resets failure count.
				// Success in half-open transitions to closed.
				// In any case, consecutive failures reset.
				if cb.State() == CircuitClosed {
					consecutiveFailures = 0
				} else if cb.State() == CircuitHalfOpen {
					// After success in half-open, state becomes closed.
					consecutiveFailures = 0
				}
			case 1: // RecordFailure
				cb.RecordFailure()
				state := cb.State()
				if state == CircuitClosed || state == CircuitHalfOpen {
					consecutiveFailures++
				}
			}

			// Core invariant: if consecutive failures >= threshold, Allow() must be false.
			if consecutiveFailures >= threshold {
				if cb.Allow() {
					t.Fatalf("invariant violated: after %d consecutive failures (threshold=%d), Allow() returned true; state=%q",
						consecutiveFailures, threshold, cb.State())
				}
			}
		}
	})
}

// TestCircuitBreaker_P8_SuccessResetInvariant verifies that a success in
// closed state always resets the failure counter, so threshold failures
// are needed again to open the circuit.
func TestCircuitBreaker_P8_SuccessResetInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		threshold := rapid.IntRange(2, 10).Draw(t, "threshold")
		failsBefore := rapid.IntRange(1, threshold-1).Draw(t, "failsBefore")

		cb := NewCircuitBreaker("pbt-reset", threshold, 1*time.Hour)

		// Record some failures below threshold.
		for i := 0; i < failsBefore; i++ {
			cb.RecordFailure()
		}

		// Record a success — should reset.
		cb.RecordSuccess()

		// Now record threshold-1 failures — should still be closed.
		for i := 0; i < threshold-1; i++ {
			cb.RecordFailure()
		}

		if cb.State() != CircuitClosed {
			t.Fatalf("expected closed after success reset + %d failures (threshold=%d), got %q",
				threshold-1, threshold, cb.State())
		}
		if !cb.Allow() {
			t.Fatal("expected Allow() = true when below threshold after reset")
		}
	})
}
