package retry

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultStrategy(t *testing.T) {
	strategy := DefaultStrategy()

	assert.Equal(t, 10, strategy.MaxAttempts)
	assert.Equal(t, 30*time.Second, strategy.BaseDelay)
	assert.Equal(t, 30*time.Minute, strategy.MaxDelay)
	assert.Equal(t, 2.0, strategy.ExponentialBase)
	assert.Equal(t, 5, strategy.DLQThreshold)
}

func TestStrategy_CalculateRetryDelay(t *testing.T) {
	strategy := DefaultStrategy()

	tests := []struct {
		name          string
		attemptNumber int
		expectedDelay time.Duration
		description   string
	}{
		{
			name:          "Zero attempts - base delay",
			attemptNumber: 0,
			expectedDelay: 30 * time.Second,
			description:   "Should return base delay for 0 attempts",
		},
		{
			name:          "First attempt - base delay",
			attemptNumber: 1,
			expectedDelay: 60 * time.Second, // 30s * 2^1
			description:   "Should double the base delay",
		},
		{
			name:          "Second attempt - exponential",
			attemptNumber: 2,
			expectedDelay: 120 * time.Second, // 30s * 2^2
			description:   "Should continue exponential growth",
		},
		{
			name:          "Third attempt",
			attemptNumber: 3,
			expectedDelay: 240 * time.Second, // 30s * 2^3 = 4 minutes
			description:   "Should be 4 minutes",
		},
		{
			name:          "Fourth attempt",
			attemptNumber: 4,
			expectedDelay: 480 * time.Second, // 30s * 2^4 = 8 minutes
			description:   "Should be 8 minutes",
		},
		{
			name:          "Fifth attempt",
			attemptNumber: 5,
			expectedDelay: 960 * time.Second, // 30s * 2^5 = 16 minutes
			description:   "Should be 16 minutes",
		},
		{
			name:          "Sixth attempt - capped",
			attemptNumber: 6,
			expectedDelay: 30 * time.Minute, // Would be 32min, but capped at 30min
			description:   "Should be capped at max delay",
		},
		{
			name:          "Large attempt number - still capped",
			attemptNumber: 100,
			expectedDelay: 30 * time.Minute,
			description:   "Should still be capped at max delay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := strategy.CalculateRetryDelay(tt.attemptNumber)
			assert.Equal(t, tt.expectedDelay, delay, tt.description)
		})
	}
}

func TestStrategy_CalculateRetryDelay_CustomStrategy(t *testing.T) {
	strategy := Strategy{
		MaxAttempts:     5,
		BaseDelay:       1 * time.Second,
		MaxDelay:        10 * time.Second,
		ExponentialBase: 3.0, // Triple each time
		DLQThreshold:    3,
	}

	tests := []struct {
		attemptNumber int
		expectedDelay time.Duration
	}{
		{0, 1 * time.Second},  // Base
		{1, 3 * time.Second},  // 1s * 3^1
		{2, 9 * time.Second},  // 1s * 3^2
		{3, 10 * time.Second}, // Would be 27s, but capped at 10s
		{4, 10 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		delay := strategy.CalculateRetryDelay(tt.attemptNumber)
		assert.Equal(t, tt.expectedDelay, delay)
	}
}

func TestStrategy_ShouldMoveToDLQ(t *testing.T) {
	strategy := DefaultStrategy()

	tests := []struct {
		name         string
		attemptCount int
		expected     bool
	}{
		{
			name:         "No attempts yet",
			attemptCount: 0,
			expected:     false,
		},
		{
			name:         "Below threshold",
			attemptCount: 4,
			expected:     false,
		},
		{
			name:         "At threshold",
			attemptCount: 5,
			expected:     true,
		},
		{
			name:         "Above threshold",
			attemptCount: 7,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.ShouldMoveToDLQ(tt.attemptCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStrategy_IsRetryable(t *testing.T) {
	strategy := DefaultStrategy()

	tests := []struct {
		name         string
		attemptCount int
		expected     bool
	}{
		{
			name:         "No attempts",
			attemptCount: 0,
			expected:     true,
		},
		{
			name:         "Few attempts",
			attemptCount: 5,
			expected:     true,
		},
		{
			name:         "At max attempts",
			attemptCount: 10,
			expected:     false,
		},
		{
			name:         "Beyond max attempts",
			attemptCount: 15,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.IsRetryable(tt.attemptCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStrategy_GetRetrySchedule(t *testing.T) {
	strategy := Strategy{
		MaxAttempts:     5,
		BaseDelay:       10 * time.Second,
		MaxDelay:        2 * time.Minute,
		ExponentialBase: 2.0,
		DLQThreshold:    3,
	}

	schedule := strategy.GetRetrySchedule()

	// Check that schedule contains expected elements
	assert.Contains(t, schedule, "Retry Schedule:")
	assert.Contains(t, schedule, "Attempt 1")
	assert.Contains(t, schedule, "Attempt 2")
	assert.Contains(t, schedule, "Attempt 3")
	assert.Contains(t, schedule, "Attempt 4")
	assert.Contains(t, schedule, "Attempt 5")
	assert.Contains(t, schedule, "→ Move to DLQ")

	// Check timing progression
	assert.Contains(t, schedule, "20s")   // Attempt 1: 10s * 2^1
	assert.Contains(t, schedule, "40s")   // Attempt 2: 10s * 2^2
	assert.Contains(t, schedule, "1m20s") // Attempt 3: 10s * 2^3 = 80s

	// Split into lines for more detailed verification
	lines := strings.Split(schedule, "\n")
	assert.True(t, len(lines) > 5, "Should have multiple lines")
}

func TestStrategy_GetRetrySchedule_DefaultStrategy(t *testing.T) {
	strategy := DefaultStrategy()

	schedule := strategy.GetRetrySchedule()

	// Verify structure
	assert.Contains(t, schedule, "Retry Schedule:")

	// Verify all attempts are listed
	for i := 1; i <= 10; i++ {
		assert.Contains(t, schedule, "Attempt")
	}

	// Verify DLQ marker appears
	assert.Contains(t, schedule, "→ Move to DLQ")

	// Verify some expected delays
	assert.Contains(t, schedule, "1m0s")  // Attempt 1: 30s * 2
	assert.Contains(t, schedule, "2m0s")  // Attempt 2: 30s * 4
	assert.Contains(t, schedule, "4m0s")  // Attempt 3: 30s * 8
	assert.Contains(t, schedule, "30m0s") // Max delay appears
}

// Integration test - realistic retry flow.
func TestStrategy_RealisticRetryFlow(t *testing.T) {
	strategy := DefaultStrategy()

	// Simulate a message that fails multiple times
	var delays []time.Duration

	for attempt := 1; attempt <= 10; attempt++ {
		delay := strategy.CalculateRetryDelay(attempt)
		delays = append(delays, delay)

		// Check if should be retried
		canRetry := strategy.IsRetryable(attempt)

		// Check if should move to DLQ
		shouldDLQ := strategy.ShouldMoveToDLQ(attempt)

		if attempt < 10 {
			assert.True(t, canRetry, "Should be retryable for attempt %d", attempt)
		} else {
			assert.False(t, canRetry, "Should not be retryable at max attempts")
		}

		if attempt >= 5 {
			assert.True(t, shouldDLQ, "Should move to DLQ at attempt %d", attempt)
		} else {
			assert.False(t, shouldDLQ, "Should not move to DLQ before threshold")
		}
	}

	// Verify delays are monotonically increasing until cap
	for i := 1; i < len(delays); i++ {
		assert.True(t, delays[i] >= delays[i-1],
			"Delay for attempt %d (%v) should be >= previous (%v)",
			i+1, delays[i], delays[i-1])
	}

	// Verify cap is applied
	lastDelay := delays[len(delays)-1]
	assert.Equal(t, 30*time.Minute, lastDelay, "Last delay should be capped at max")
}

// Boundary value tests.
func TestStrategy_BoundaryValues(t *testing.T) {
	t.Run("Zero base delay", func(t *testing.T) {
		strategy := Strategy{
			BaseDelay:       0,
			ExponentialBase: 2.0,
			MaxDelay:        1 * time.Minute,
		}

		delay := strategy.CalculateRetryDelay(5)
		assert.Equal(t, time.Duration(0), delay)
	})

	t.Run("Exponential base of 1", func(t *testing.T) {
		strategy := Strategy{
			BaseDelay:       30 * time.Second,
			ExponentialBase: 1.0,
			MaxDelay:        1 * time.Minute,
		}

		delay1 := strategy.CalculateRetryDelay(1)
		delay5 := strategy.CalculateRetryDelay(5)
		assert.Equal(t, delay1, delay5, "Delay should not increase with base 1.0")
	})

	t.Run("Max delay equals base delay", func(t *testing.T) {
		strategy := Strategy{
			BaseDelay:       30 * time.Second,
			ExponentialBase: 2.0,
			MaxDelay:        30 * time.Second, // Same as base
		}

		delay1 := strategy.CalculateRetryDelay(1)
		assert.Equal(t, 30*time.Second, delay1, "Should be capped at max immediately")
	})
}

// Performance test - ensure calculation is fast.
func BenchmarkCalculateRetryDelay(b *testing.B) {
	strategy := DefaultStrategy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.CalculateRetryDelay(i % 10)
	}
}

func BenchmarkShouldMoveToDLQ(b *testing.B) {
	strategy := DefaultStrategy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.ShouldMoveToDLQ(i % 10)
	}
}
