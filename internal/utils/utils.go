package utils

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strconv"
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

// ParseTime parses time string
func ParseTime(timeStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05Z07:00", // RFC3339
		"2006-01-02T15:04:05Z",      // UTC
		"2006-01-02 15:04:05",       // DateTime
		"2006-01-02",                // Date
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	if unixTime, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
		// Unix timestamp
		if unixTime > 1e10 { // Unix timestamp in milliseconds
			return time.UnixMilli(unixTime), nil
		}
		return time.Unix(unixTime, 0), nil
	}

	return time.Time{}, fmt.Errorf("unsupported time format: %s", timeStr)
}
