package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSucceedsFirstAttempt(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, Multiplier: 2.0}
	calls := 0

	err := Do(context.Background(), cfg, "test-op", func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDoRetryThenSuccess(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, Multiplier: 2.0}
	calls := 0

	err := Do(context.Background(), cfg, "test-op", func() error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoAllAttemptsFail(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, Multiplier: 2.0}
	calls := 0

	err := Do(context.Background(), cfg, "test-op", func() error {
		calls++
		return errors.New("persistent error")
	})

	if err == nil {
		t.Fatal("expected error after all attempts fail")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoContextCancellation(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: 1 * time.Second, Multiplier: 2.0}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := Do(ctx, cfg, "test-op", func() error {
		return errors.New("fail")
	})

	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestCalculateDelay(t *testing.T) {
	cfg := RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		Jitter:     0,
	}

	// Attempt 1: 1s
	d1 := calculateDelay(cfg, 1)
	if d1 != 1*time.Second {
		t.Errorf("attempt 1: expected 1s, got %v", d1)
	}

	// Attempt 2: 2s
	d2 := calculateDelay(cfg, 2)
	if d2 != 2*time.Second {
		t.Errorf("attempt 2: expected 2s, got %v", d2)
	}

	// Attempt 3: 4s
	d3 := calculateDelay(cfg, 3)
	if d3 != 4*time.Second {
		t.Errorf("attempt 3: expected 4s, got %v", d3)
	}
}

func TestCalculateDelayMaxCap(t *testing.T) {
	cfg := RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Second,
		Multiplier: 10.0,
		Jitter:     0,
	}

	d := calculateDelay(cfg, 3)
	if d != 5*time.Second {
		t.Errorf("expected capped at 5s, got %v", d)
	}
}

func TestCalculateDelayWithJitter(t *testing.T) {
	cfg := RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.5,
	}

	// With 50% jitter, delay for attempt 1 should be between 0.5s and 1.5s
	for i := 0; i < 20; i++ {
		d := calculateDelay(cfg, 1)
		if d < 500*time.Millisecond || d > 1500*time.Millisecond {
			t.Errorf("delay %v out of expected jitter range [500ms, 1500ms]", d)
		}
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay 1s, got %v", cfg.BaseDelay)
	}
}
