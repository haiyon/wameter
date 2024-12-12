package notify

import (
	"sync"
	"time"
)

// RateLimiter implements rate limiting for notifications
type RateLimiter struct {
	mu        sync.Mutex
	events    map[NotifierType][]time.Time
	interval  time.Duration
	maxEvents int
}

// AllowNotification checks if a notification is allowed under rate limits
func (r *RateLimiter) AllowNotification(notifierType NotifierType) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	timestamps := r.events[notifierType]

	// Clean expired timestamps
	valid := make([]time.Time, 0)
	for _, ts := range timestamps {
		if now.Sub(ts) < r.interval {
			valid = append(valid, ts)
		}
	}
	r.events[notifierType] = valid

	// Check if limit exceeded
	if len(valid) >= r.maxEvents {
		return false
	}

	// Add new timestamp
	r.events[notifierType] = append(r.events[notifierType], now)
	return true
}
