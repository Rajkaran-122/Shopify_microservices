// Package retry implements exponential backoff with jitter for resilient
// inter-service communication. Per BRD Section 7.5: base 100ms, multiplier 2,
// max 30s, max 3 retries for idempotent operations only.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Config defines retry behavior per BRD specifications.
type Config struct {
	MaxRetries    int           // BRD: 3
	InitialDelay  time.Duration // BRD: 100ms
	MaxDelay      time.Duration // BRD: 30s
	Multiplier    float64       // BRD: 2.0
	JitterFactor  float64       // 0.0-1.0, adds randomness to prevent thundering herd
}

// DefaultConfig returns retry configuration matching BRD Section 7.5.
func DefaultConfig() Config {
	return Config{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.5,
	}
}

// RetryableFunc is a function that can be retried.
type RetryableFunc func(ctx context.Context) error

// IsRetryable determines if an error should be retried.
type IsRetryable func(err error) bool

// DefaultIsRetryable retries all errors except context cancellation.
func DefaultIsRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

// Do executes a function with exponential backoff retry logic.
// Only idempotent operations should be retried per BRD requirements.
//
// The delay between retries follows: delay = min(initialDelay * multiplier^attempt, maxDelay) + jitter
func Do(ctx context.Context, cfg Config, fn RetryableFunc, isRetryable IsRetryable) error {
	if isRetryable == nil {
		isRetryable = DefaultIsRetryable
	}

	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil // Success
		}

		// Don't retry if the error is non-retryable
		if !isRetryable(lastErr) {
			return lastErr
		}

		// Don't wait after the last attempt
		if attempt == cfg.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(cfg, attempt)

		// Wait with context awareness
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	return lastErr
}

// calculateDelay computes the delay for a given attempt with jitter.
func calculateDelay(cfg Config, attempt int) time.Duration {
	// Exponential: initialDelay * multiplier^attempt
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt))

	// Cap at maximum delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Add jitter to prevent thundering herd problem
	if cfg.JitterFactor > 0 {
		jitter := delay * cfg.JitterFactor * (rand.Float64()*2 - 1) // ±jitterFactor
		delay += jitter
		if delay < 0 {
			delay = float64(cfg.InitialDelay) // Floor at initial delay
		}
	}

	return time.Duration(delay)
}
