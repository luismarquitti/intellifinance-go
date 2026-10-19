package main

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/hibiken/asynq"
)

// CalculateRetryDelay uses Asynq's zero-based retry count: the first retry
// waits 5 seconds, with +/-20% jitter and a hard cap of 5 minutes.
func CalculateRetryDelay(n int, err error, _ *asynq.Task) time.Duration {
	if errors.Is(err, asynq.SkipRetry) {
		return 0
	}
	base := 5 * time.Second
	for i := 0; i < n && base < 300*time.Second; i++ {
		base *= 2
	}
	if base > 300*time.Second {
		base = 300 * time.Second
	}
	span := base / 5
	// Package-level rand/v2 functions are safe for concurrent use. This is
	// scheduling jitter, so no cryptographic randomness is needed.
	delay := base - span + time.Duration(rand.Int64N(int64(2*span)+1)) //nolint:gosec // Non-security scheduling jitter.
	if delay > 300*time.Second {
		return 300 * time.Second
	}
	return delay
}
