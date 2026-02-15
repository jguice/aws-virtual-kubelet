package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

const (
	// StateClosed means the circuit is functioning normally (requests pass through).
	StateClosed CircuitState = iota
	// StateOpen means the circuit has tripped (requests are rejected immediately).
	StateOpen
	// StateHalfOpen means the circuit is testing if the downstream has recovered.
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreakerConfig defines parameters for a circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures to trip the circuit.
	FailureThreshold int
	// ResetTimeout is how long the circuit stays open before transitioning to half-open.
	ResetTimeout time.Duration
	// HalfOpenMaxAttempts is how many test requests are allowed in half-open state.
	HalfOpenMaxAttempts int
}

// DefaultCircuitBreakerConfig returns a reasonable default configuration.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    5,
		ResetTimeout:        60 * time.Second,
		HalfOpenMaxAttempts: 1,
	}
}

// CircuitBreaker implements the circuit breaker pattern for protecting
// downstream services from cascading failures.
type CircuitBreaker struct {
	mu sync.RWMutex

	name   string
	config CircuitBreakerConfig

	state            CircuitState
	failures         int
	lastFailureTime  time.Time
	halfOpenAttempts int
}

// NewCircuitBreaker creates a new circuit breaker with the given name and config.
func NewCircuitBreaker(name string, cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:   name,
		config: cfg,
		state:  StateClosed,
	}
}

// Execute runs fn through the circuit breaker. If the circuit is open, it returns
// ErrCircuitOpen immediately. On success, it resets the failure counter. On failure,
// it increments the failure counter and may trip the circuit.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if err := cb.allowRequest(); err != nil {
		return err
	}

	err := fn()

	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// Check if open circuit has timed out (should transition to half-open)
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.config.ResetTimeout {
		return StateHalfOpen
	}
	return cb.state
}

// Failures returns the current consecutive failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Reset manually resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenAttempts = 0
	klog.Infof("circuit breaker %q manually reset to closed", cb.name)
}

func (cb *CircuitBreaker) allowRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) > cb.config.ResetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenAttempts = 0
			klog.Infof("circuit breaker %q transitioning from open to half-open", cb.name)
			return nil
		}
		return fmt.Errorf("%w: %s (failures: %d, retry after %v)",
			ErrCircuitOpen, cb.name, cb.failures,
			cb.config.ResetTimeout-time.Since(cb.lastFailureTime))

	case StateHalfOpen:
		if cb.halfOpenAttempts >= cb.config.HalfOpenMaxAttempts {
			return fmt.Errorf("%w: %s (half-open, max test attempts reached)", ErrCircuitOpen, cb.name)
		}
		cb.halfOpenAttempts++
		return nil
	}

	return nil
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = StateOpen
			klog.Warningf("circuit breaker %q tripped (open) after %d consecutive failures",
				cb.name, cb.failures)
		}
	case StateHalfOpen:
		// Test request failed in half-open state, reopen
		cb.state = StateOpen
		klog.Warningf("circuit breaker %q reopened after failed half-open test", cb.name)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	prevState := cb.state

	cb.failures = 0
	cb.state = StateClosed
	cb.halfOpenAttempts = 0

	if prevState != StateClosed {
		klog.Infof("circuit breaker %q closed after successful request (was %s)", cb.name, prevState)
	}
}
