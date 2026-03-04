package middleware

import (
	"sync"

	"github.com/gofiber/fiber/v2"
)

type ConcurrencyLimiter struct {
	semaphore chan struct{}
}

func NewConcurrencyLimiter(maxConcurrent int) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		semaphore: make(chan struct{}, maxConcurrent),
	}
}

func (cl *ConcurrencyLimiter) Handle() fiber.Handler {
	return func(c *fiber.Ctx) error {
		cl.semaphore <- struct{}{}
		defer func() { <-cl.semaphore }()
		return c.Next()
	}
}

type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]int64
	limit    int
	window   int64
}

func NewRateLimiter(limit int, windowSeconds int) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]int64),
		limit:    limit,
		window:   int64(windowSeconds),
	}
}

func (rl *RateLimiter) Handle() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rl.mu.Lock()
		defer rl.mu.Unlock()

		key := c.IP()
		now := int64(c.Context().Time().Unix())

		windowStart := now - rl.window
		requests := rl.requests[key]

		var validRequests []int64
		for _, ts := range requests {
			if ts > windowStart {
				validRequests = append(validRequests, ts)
			}
		}

		if len(validRequests) >= rl.limit {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
			})
		}

		rl.requests[key] = append(validRequests, now)
		return c.Next()
	}
}
