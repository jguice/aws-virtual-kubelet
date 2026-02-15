package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerStartsClosed(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultCircuitBreakerConfig())
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State())
	}
	if cb.Failures() != 0 {
		t.Errorf("expected 0 failures, got %d", cb.Failures())
	}
}

func TestCircuitBreakerPassesOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultCircuitBreakerConfig())
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State())
	}
}

func TestCircuitBreakerTripsAfterThreshold(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    3,
		ResetTimeout:        1 * time.Second,
		HalfOpenMaxAttempts: 1,
	}
	cb := NewCircuitBreaker("test", cfg)

	testErr := errors.New("service down")

	// First 3 failures should pass through
	for i := 0; i < 3; i++ {
		err := cb.Execute(context.Background(), func() error {
			return testErr
		})
		if err == nil {
			t.Fatalf("expected error on attempt %d", i+1)
		}
	}

	// Circuit should now be open
	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after %d failures, got %v", cfg.FailureThreshold, cb.State())
	}

	// Next call should be rejected immediately
	err := cb.Execute(context.Background(), func() error {
		t.Error("function should not be called when circuit is open")
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxAttempts: 1,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Trip the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return errors.New("fail")
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen, got %v", cb.State())
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// State should now be half-open
	if cb.State() != StateHalfOpen {
		t.Errorf("expected StateHalfOpen after timeout, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenSuccess(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxAttempts: 1,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Trip the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return errors.New("fail")
		})
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Half-open test request succeeds → circuit closes
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error in half-open, got: %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after half-open success, got %v", cb.State())
	}
	if cb.Failures() != 0 {
		t.Errorf("expected 0 failures after reset, got %d", cb.Failures())
	}
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxAttempts: 1,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Trip the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return errors.New("fail")
		})
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Half-open test request fails → circuit reopens
	cb.Execute(context.Background(), func() error {
		return errors.New("still broken")
	})

	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreakerSuccessResetsFailures(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    5,
		ResetTimeout:        1 * time.Second,
		HalfOpenMaxAttempts: 1,
	}
	cb := NewCircuitBreaker("test", cfg)

	// 3 failures (below threshold)
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func() error {
			return errors.New("fail")
		})
	}
	if cb.Failures() != 3 {
		t.Errorf("expected 3 failures, got %d", cb.Failures())
	}

	// Success resets counter
	cb.Execute(context.Background(), func() error {
		return nil
	})
	if cb.Failures() != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.Failures())
	}
}

func TestCircuitBreakerManualReset(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		ResetTimeout:        1 * time.Hour,
		HalfOpenMaxAttempts: 1,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Trip the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return errors.New("fail")
		})
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen")
	}

	// Manual reset
	cb.Reset()
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after reset, got %v", cb.State())
	}
	if cb.Failures() != 0 {
		t.Errorf("expected 0 failures after reset, got %d", cb.Failures())
	}
}

func TestCircuitStateString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, tt.state.String())
		}
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	if cfg.FailureThreshold != 5 {
		t.Errorf("expected FailureThreshold 5, got %d", cfg.FailureThreshold)
	}
	if cfg.ResetTimeout != 60*time.Second {
		t.Errorf("expected ResetTimeout 60s, got %v", cfg.ResetTimeout)
	}
}
