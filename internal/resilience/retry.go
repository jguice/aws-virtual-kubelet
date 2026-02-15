package resilience

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"k8s.io/klog/v2"
)

// RetryConfig defines parameters for exponential backoff retry.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the initial call).
	MaxAttempts int
	// BaseDelay is the initial delay between retries.
	BaseDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
	// Multiplier is the factor by which the delay increases after each attempt.
	Multiplier float64
	// Jitter adds randomness to delays to prevent thundering herd.
	Jitter float64
}

// DefaultRetryConfig returns a reasonable default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      0.3,
	}
}

// Do executes fn with exponential backoff retry. It calls fn up to cfg.MaxAttempts
// times, returning the first successful result or the last error.
func Do(ctx context.Context, cfg RetryConfig, operation string, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			if attempt > 1 {
				klog.V(1).Infof("retry succeeded on attempt %d/%d for %s",
					attempt, cfg.MaxAttempts, operation)
			}
			return nil
		}

		if attempt == cfg.MaxAttempts {
			break
		}

		// Check context before sleeping
		if ctx.Err() != nil {
			return fmt.Errorf("%s failed (context cancelled after attempt %d/%d): %w",
				operation, attempt, cfg.MaxAttempts, lastErr)
		}

		delay := calculateDelay(cfg, attempt)
		klog.Warningf("attempt %d/%d for %s failed: %v (retrying in %v)",
			attempt, cfg.MaxAttempts, operation, lastErr, delay)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return fmt.Errorf("%s failed (context cancelled during backoff after attempt %d/%d): %w",
				operation, attempt, cfg.MaxAttempts, lastErr)
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w",
		operation, cfg.MaxAttempts, lastErr)
}

func calculateDelay(cfg RetryConfig, attempt int) time.Duration {
	delay := float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Apply jitter
	if cfg.Jitter > 0 {
		jitterRange := delay * cfg.Jitter
		delay = delay - jitterRange + (rand.Float64() * 2 * jitterRange) //nolint:gosec
	}

	return time.Duration(delay)
}
