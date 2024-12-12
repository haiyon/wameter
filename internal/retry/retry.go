package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Func defines the function signature for a retryable operation
type Func func(ctx context.Context) error

// LoggerFunc defines a logging function signature
type LoggerFunc func(format string, args ...any)

// Default logger (can be replaced by a custom logger)
var logger LoggerFunc = func(format string, args ...any) {
	fmt.Printf(format, args...)
}

// SetLogger allows setting a custom logger for retry operations
func SetLogger(customLogger LoggerFunc) {
	logger = customLogger
}

// Execute performs an operation with a retry mechanism.
//
// Usage:
//
//	// Simple retry with default config
//	err := retry.Execute(ctx, retry.DefaultRetryConfig(), func(ctx context.Context) error {
//	    return someOperation()
//	})
//
//	// Custom retry stages
//	config := &retry.Config{
//	    Enabled:        true,
//	    EnabledStages: retry.StageInitial | retry.StageMinute, // Only initial and minute retries
//	    InitialAttempts: 3,
//	    InitialInterval: time.Second,
//	    MinuteAttempts:  30,
//	    MinuteInterval:  time.Minute,
//	}
//	err := retry.Execute(ctx, config, func(ctx context.Context) error {
//	    return someOperation()
//	})
//
//	// Stop retry on specific errors
//	err := retry.Execute(ctx, config, func(ctx context.Context) error {
//	    err := someOperation()
//	    if err != nil {
//	        if errors.Is(err, sql.ErrNoRows) { // example of specific error check
//	            return retry.StopRetry(err) // Stop retry immediately
//	        }
//	        return err // Continue retry on other errors
//	    }
//	    return nil
//	})
func Execute(ctx context.Context, cfg *Config, op Func) error {
	// If no retry configuration is provided, just execute the operation
	if cfg == nil || !cfg.Enabled {
		return op(ctx)
	}

	// Validate retry configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid retry configuration: %w", err)
	}

	var lastErr error

	// Initial fast retries
	if cfg.Stage&StageInitial != 0 && cfg.InitialAttempts > 0 {
		if err := executeStage(ctx, cfg.InitialAttempts, cfg.InitialInterval, op); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	// Minute-interval retries
	if cfg.Stage&StageMinute != 0 && cfg.MinuteAttempts > 0 {
		if err := executeStage(ctx, cfg.MinuteAttempts, cfg.MinuteInterval, op); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	// Hour-interval retries
	if cfg.Stage&StageHourly != 0 && cfg.HourlyAttempts > 0 {
		if err := executeStage(ctx, cfg.HourlyAttempts, cfg.HourlyInterval, op); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	// Final retry attempt with timeout
	if cfg.Stage&StageFinal != 0 && cfg.FinalRetryTimeout > 0 {
		finalCtx, cancel := context.WithTimeout(ctx, cfg.FinalRetryTimeout)
		defer cancel()
		if err := op(finalCtx); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("operation failed after all retries: %v", lastErr)
}

// executeStage executes a single retry stage
func executeStage(ctx context.Context, attempts int, interval time.Duration, op Func) error {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		if err := op(ctx); err == nil {
			return nil
		} else {
			lastErr = err
			logger("Retry %d/%d failed: %v. Waiting %v before next attempt...\n",
				i, attempts, err, interval)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}

	return fmt.Errorf("exhausted %d attempts: %v", attempts, lastErr)
}

// StopRetryError is a special error type that indicates retry should stop
type StopRetryError struct {
	err error
}

func (e *StopRetryError) Error() string {
	return e.err.Error()
}

// StopRetry wraps an error to indicate that retry should stop
func StopRetry(err error) error {
	if err == nil {
		return nil
	}
	return &StopRetryError{err: err}
}

// IsStopRetry checks if an error is a StopRetryError
func IsStopRetry(err error) bool {
	var stopRetryError *StopRetryError
	ok := errors.As(err, &stopRetryError)
	return ok
}
