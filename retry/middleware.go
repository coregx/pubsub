// Package retry provides exponential backoff retry strategies for message delivery.
// It implements configurable retry logic with Dead Letter Queue threshold for permanent failures.
package retry

import (
	"fmt"
	"math"
	"time"
)

// Strategy defines the retry behavior configuration for failed message deliveries.
// It implements exponential backoff with configurable parameters.
//
// The retry schedule follows: delay = min(BaseDelay * ExponentialBase^attempt, MaxDelay)
//
// Example with defaults (30s base, 2.0 exponential, 30m max):
//
//	Attempt 1: 30s
//	Attempt 2: 1m
//	Attempt 3: 2m
//	Attempt 4: 4m
//	Attempt 5: 8m (→ DLQ)
type Strategy struct {
	MaxAttempts     int           // Maximum retry attempts before giving up
	BaseDelay       time.Duration // Initial retry delay (first attempt)
	MaxDelay        time.Duration // Maximum retry delay cap
	ExponentialBase float64       // Backoff multiplier (e.g., 2.0 for doubling)
	DLQThreshold    int           // Move to Dead Letter Queue after this many attempts
}

// DefaultStrategy returns the production-ready default retry strategy.
// Configuration: 10 max attempts, 30s→30m exponential backoff, DLQ after 5 attempts.
//
// This strategy has been battle-tested in the FreiCON Railway Management System.
func DefaultStrategy() Strategy {
	return Strategy{
		MaxAttempts:     10,
		BaseDelay:       30 * time.Second,
		MaxDelay:        30 * time.Minute,
		ExponentialBase: 2.0,
		DLQThreshold:    5,
	}
}

// CalculateRetryDelay calculates the retry delay for a given attempt using exponential backoff.
// Formula: delay = min(BaseDelay * ExponentialBase^attemptNumber, MaxDelay)
//
// Parameters:
//   - attemptNumber: The attempt number (0-based or 1-based depending on usage)
//
// Returns the delay duration to wait before the next retry attempt.
func (s Strategy) CalculateRetryDelay(attemptNumber int) time.Duration {
	if attemptNumber <= 0 {
		return s.BaseDelay
	}

	// Calculate exponential delay
	delay := float64(s.BaseDelay) * math.Pow(s.ExponentialBase, float64(attemptNumber))

	// Cap at max delay
	if delay > float64(s.MaxDelay) {
		return s.MaxDelay
	}

	return time.Duration(delay)
}

// ShouldMoveToDLQ determines if a message should be moved to the Dead Letter Queue.
// Returns true when the attempt count reaches or exceeds the DLQ threshold.
func (s Strategy) ShouldMoveToDLQ(attemptCount int) bool {
	return attemptCount >= s.DLQThreshold
}

// IsRetryable checks if another retry attempt is allowed.
// Returns true if the attempt count is below the maximum attempts limit.
func (s Strategy) IsRetryable(attemptCount int) bool {
	return attemptCount < s.MaxAttempts
}

// GetRetrySchedule returns a human-readable description of the retry schedule.
// Useful for debugging, documentation, and displaying retry behavior to users.
//
// Example output:
//
//	Retry Schedule:
//	  Attempt 1: after 30s
//	  Attempt 2: after 1m
//	  ...
//	  Attempt 5: after 8m
//	  → Move to DLQ
func (s Strategy) GetRetrySchedule() string {
	schedule := "Retry Schedule:\n"
	for i := 1; i <= s.MaxAttempts; i++ {
		delay := s.CalculateRetryDelay(i)
		schedule += fmt.Sprintf("  Attempt %d: after %v\n", i, delay)
		if i == s.DLQThreshold {
			schedule += "  → Move to DLQ\n"
		}
	}
	return schedule
}
