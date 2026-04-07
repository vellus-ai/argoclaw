package plugins

import (
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// CircuitBreaker — Task 9.8
// Implements the circuit breaker pattern for plugin tool calls.
// Prevents request accumulation against degraded plugins.
// Validates: Requirement 24.2
// ─────────────────────────────────────────────────────────────────────────────

// CircuitState represents the state of a circuit breaker.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half-open"
)

// CircuitBreaker implements the circuit breaker pattern for tool calls.
// It tracks consecutive failures and transitions between closed, open,
// and half-open states to protect the system from cascading failures.
//
// States:
//   - Closed: all calls allowed; failures are counted.
//   - Open: all calls rejected; after resetTimeout, transitions to half-open.
//   - Half-open: one probe call allowed; success closes, failure re-opens.
type CircuitBreaker struct {
	name         string
	state        CircuitState
	failureCount int
	threshold    int
	resetTimeout time.Duration
	lastFailure  time.Time
	mu           sync.Mutex
}

// NewCircuitBreaker creates a CircuitBreaker with the given threshold and
// reset timeout. The circuit starts in the closed state.
func NewCircuitBreaker(name string, threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		state:        CircuitClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// State returns the current circuit state. This is safe for concurrent use.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentState()
}

// Allow checks whether a call is permitted through the circuit breaker.
//
//   - Closed: always allows.
//   - Open: rejects until resetTimeout has elapsed, then transitions to
//     half-open and allows one probe call.
//   - Half-open: rejects (only one probe is allowed per half-open window).
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.currentState() {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if enough time has passed to transition to half-open.
		if time.Since(cb.lastFailure) >= cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return true // allow one probe call
		}
		return false
	case CircuitHalfOpen:
		// Only one probe call is allowed; subsequent calls are rejected
		// until the probe result is recorded.
		return false
	default:
		return false
	}
}

// RecordSuccess records a successful call.
//   - In closed state: resets the failure counter.
//   - In half-open state: transitions to closed (circuit recovered).
//   - In open state: no-op (shouldn't happen, but safe).
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.state = CircuitClosed
		cb.failureCount = 0
	case CircuitClosed:
		cb.failureCount = 0
	}
}

// RecordFailure records a failed call.
//   - In closed state: increments failure count; opens circuit at threshold.
//   - In half-open state: transitions back to open.
//   - In open state: updates lastFailure timestamp.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failureCount++
		if cb.failureCount >= cb.threshold {
			cb.state = CircuitOpen
		}
	case CircuitHalfOpen:
		cb.state = CircuitOpen
	case CircuitOpen:
		// Already open — just update lastFailure for timeout tracking.
	}
}

// currentState returns the effective state, accounting for timeout-based
// transitions. Must be called with mu held.
func (cb *CircuitBreaker) currentState() CircuitState {
	if cb.state == CircuitOpen && time.Since(cb.lastFailure) >= cb.resetTimeout {
		// Don't auto-transition here; let Allow() handle it so we can
		// control the single-probe semantics.
		return CircuitOpen
	}
	return cb.state
}
