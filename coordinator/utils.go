package main

import (
	"math"
	"math/rand"
	"time"
)

func calculateBackoffDelay(retryCount int) time.Duration {
	// Exponentail Backoff: 2 ^ retryCount seconds with jitter
	baseDelay := time.Duration(math.Pow(2, float64(retryCount))) * time.Second

	// Add jitter to prevent thundering herd
	// Having fixed delay may cause multiple failure jobs access same resources at exactly same time
	// This causes a sudden spike and it can overwhelm the system.
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond

	// Cap at maximum delay
	maxDelay := 60 * time.Second
	if baseDelay > maxDelay {
		baseDelay = maxDelay
	}

	return baseDelay + jitter
}
