package retry

import (
	"context"
	"fmt"
	"time"
)

// Func defines the function signature for a retryable operation.
type Func func(ctx context.Context) error

// LoggerFunc defines a logging function signature.
type LoggerFunc func(format string, args ...interface{})

// Default logger (can be replaced by a custom logger)
var logger LoggerFunc = func(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// SetLogger allows setting a custom logger for retry operations.
func SetLogger(customLogger LoggerFunc) {
	logger = customLogger
}

// Execute performs an operation with a retry mechanism.
func Execute(ctx context.Context, cfg *Config, op Func) error {
	// If no retry configuration is provided, just execute the operation
	if cfg == nil || !cfg.Enable {
		return op(ctx)
	}
	// Validate retry configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid retry configuration: %w", err)
	}

	// Helper function to handle retries
	var lastErr error
	attemptRetry := func(attempts int, interval time.Duration) error {
		for i := 1; i <= attempts; i++ {
			if err := op(ctx); err == nil {
				return nil
			} else {
				lastErr = err
				logger("Retry %d/%d failed: %v. Waiting %v before next attempt...\n", i, attempts, err, interval)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interval):
			}
		}
		return fmt.Errorf("exhausted %d attempts", attempts)
	}

	// Sequential retry levels
	retryStages := []struct {
		attempts int
		interval time.Duration
	}{
		{cfg.InitialAttempts, cfg.InitialInterval},
		{cfg.MinuteAttempts, cfg.MinuteInterval},
		{cfg.HourlyAttempts, cfg.HourlyInterval},
	}

	// Perform retries
	for _, stage := range retryStages {
		if err := attemptRetry(stage.attempts, stage.interval); err == nil {
			return nil
		}
	}

	// Final retry with timeout
	finalCtx, cancel := context.WithTimeout(ctx, cfg.FinalRetryTimeout)
	defer cancel()
	if err := op(finalCtx); err == nil {
		return nil
	}
	return fmt.Errorf("operation failed after all retries: %v", lastErr)
}
