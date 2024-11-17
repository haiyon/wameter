package utils

import (
	"fmt"
	"net"
	"sort"
	"time"
)

// IsValidIP checks if a string is a valid IP address, optionally checking for IPv6
func IsValidIP(ip string, wantV6 ...bool) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	if len(wantV6) > 0 && wantV6[0] {
		return parsedIP.To16() != nil && parsedIP.To4() == nil
	}
	return parsedIP.To4() != nil
}

// IsGlobalIPv6 checks if the IPv6 address is global (not link-local)
func IsGlobalIPv6(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	return true
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
