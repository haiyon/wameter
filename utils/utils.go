package utils

import (
	"fmt"
	"runtime"
	"sort"
	"time"
)

// IsLinux checks if the current system is Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// Retry retries an operation with exponential backoff
func Retry(maxAttempts int, initialDelay time.Duration, op func() error) error {
	var err error
	delay := initialDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = op()
		if err == nil {
			return nil
		}

		if attempt == maxAttempts {
			break
		}

		time.Sleep(delay)
		delay *= 2 // exponential backoff
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxAttempts, err)
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
