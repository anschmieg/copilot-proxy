package utils

import (
	"fmt"
	"sync"
	"time"
)

// RateLimit interface defines methods for rate limiting
type RateLimit interface {
	Capacity() int
	RefillDuration() time.Duration
	DBName() string
}

// BasicRateLimit provides a simple implementation of RateLimit
type BasicRateLimit struct {
	capacity       int
	refillDuration time.Duration
	dbName         string
}

// Capacity returns the token capacity
func (r *BasicRateLimit) Capacity() int {
	return r.capacity
}

// RefillDuration returns the time to refill one token
func (r *BasicRateLimit) RefillDuration() time.Duration {
	return r.refillDuration
}

// DBName returns the name for database storage
func (r *BasicRateLimit) DBName() string {
	return r.dbName
}

// NewBasicRateLimit creates a new basic rate limit
func NewBasicRateLimit(capacity int, refillDuration time.Duration, dbName string) *BasicRateLimit {
	return &BasicRateLimit{
		capacity:       capacity,
		refillDuration: refillDuration,
		dbName:         dbName,
	}
}

// RateBucket implements the token bucket algorithm
type RateBucket struct {
	capacity           int
	tokenCount         int
	refillTimePerToken time.Duration
	lastRefill         time.Time
}

// RateLimiter is a structure that implements a token bucket algorithm for rate limiting.
type RateLimiter struct {
	buckets      map[string]*RateBucket
	dirtyBuckets map[string]struct{}
	mu           sync.Mutex
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets:      make(map[string]*RateBucket),
		dirtyBuckets: make(map[string]struct{}),
	}
}

// Check returns an error if the user has exceeded the specified rate limit
func (rl *RateLimiter) Check(limit RateLimit, userID uint64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Create bucket key using user ID and limit name
	bucketKey := getBucketKey(userID, limit.DBName())

	// Get or create bucket
	bucket, exists := rl.buckets[bucketKey]
	if !exists {
		bucket = &RateBucket{
			capacity:           limit.Capacity(),
			tokenCount:         limit.Capacity(),
			refillTimePerToken: limit.RefillDuration() / time.Duration(limit.Capacity()),
			lastRefill:         time.Now(),
		}
		rl.buckets[bucketKey] = bucket
	}

	// Mark bucket as dirty for persistence
	rl.dirtyBuckets[bucketKey] = struct{}{}

	// Check if request can be allowed
	return bucket.Allow(time.Now())
}

// Allow determines whether a request is permitted based on the rate-limiting rules.
// It returns true if the request is allowed, otherwise false.
func (rb *RateBucket) Allow(now time.Time) bool {
	rb.Refill(now)

	if rb.tokenCount > 0 {
		rb.tokenCount--
		return true
	}

	return false
}

// Refill adds tokens to the bucket based on elapsed time
func (rb *RateBucket) Refill(now time.Time) {
	elapsed := now.Sub(rb.lastRefill)

	if elapsed >= rb.refillTimePerToken {
		newTokens := int(elapsed / rb.refillTimePerToken)
		if rb.tokenCount+newTokens > rb.capacity {
			rb.tokenCount = rb.capacity
		} else {
			rb.tokenCount += newTokens
		}

		// Calculate unused refill time
		unusedTime := elapsed % rb.refillTimePerToken
		rb.lastRefill = now.Add(-unusedTime)
	}
}

// getBucketKey creates a key for the rate limiter bucket
func getBucketKey(userID uint64, limitName string) string {
	return fmt.Sprintf("%d:%s", userID, limitName)
}
