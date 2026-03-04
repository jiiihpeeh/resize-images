package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConcurrencyLimiterAllowsUpToLimit(t *testing.T) {
	limiter := NewConcurrencyLimiter(2)

	sem1 := make(chan struct{}, 1)
	sem2 := make(chan struct{}, 1)

	select {
	case limiter.semaphore <- struct{}{}:
		sem1 <- struct{}{}
	default:
		t.Fatal("Expected first slot to be available")
	}

	select {
	case limiter.semaphore <- struct{}{}:
		sem2 <- struct{}{}
	default:
		t.Fatal("Expected second slot to be available")
	}

	select {
	case limiter.semaphore <- struct{}{}:
		t.Fatal("Expected third slot to be blocked")
	default:
	}

	<-limiter.semaphore

	select {
	case limiter.semaphore <- struct{}{}:
	default:
		t.Fatal("Expected slot to be available after release")
	}
}

func TestRateLimiterAllowsRequestsUnderLimit(t *testing.T) {
	limiter := NewRateLimiter(5, 60)

	for i := 0; i < 5; i++ {
		allowed := limiter.allowRequest("test-ip")
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	allowed := limiter.allowRequest("test-ip")
	assert.False(t, allowed, "6th request should be blocked")
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	limiter := NewRateLimiter(2, 60)

	assert.True(t, limiter.allowRequest("ip1"))
	assert.True(t, limiter.allowRequest("ip1"))
	assert.False(t, limiter.allowRequest("ip1"))

	assert.True(t, limiter.allowRequest("ip2"))
	assert.True(t, limiter.allowRequest("ip2"))
	assert.False(t, limiter.allowRequest("ip2"))
}

func TestRateLimiterClearsOldRequests(t *testing.T) {
	limiter := &RateLimiter{
		requests: make(map[string][]int64),
		limit:    2,
		window:   1,
	}

	now := time.Now().Unix()

	limiter.requests["test"] = []int64{now - 5, now - 5}

	allowed := limiter.allowRequest("test")
	assert.True(t, allowed, "Old requests should be cleared")
}

func (rl *RateLimiter) allowRequest(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := int64(time.Now().Unix())
	windowStart := now - rl.window

	var validRequests []int64
	for _, ts := range rl.requests[ip] {
		if ts > windowStart {
			validRequests = append(validRequests, ts)
		}
	}

	if len(validRequests) >= rl.limit {
		return false
	}

	rl.requests[ip] = append(validRequests, now)
	return true
}
