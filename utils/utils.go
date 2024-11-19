package utils

import (
	"errors"
	"runtime"
	"sort"
	"time"
)

// IsLinux checks if the current system is Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
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

// Retry retries an operation with exponential backoff
func Retry(attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if IsStopRetry(err) {
			return err.(*StopRetryError).err
		}
		if i < attempts-1 {
			time.Sleep(delay)
			delay = delay * 2
		}
	}
	return err
}

// StringSlicesEqual compares two string slices
func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// Sort both slices for comparison
	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))
	copy(aCopy, a)
	copy(bCopy, b)
	sort.Strings(aCopy)
	sort.Strings(bCopy)

	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}
