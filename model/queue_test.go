package model

import (
	"errors"
	"testing"
	"time"

	"database/sql"
	"github.com/stretchr/testify/assert"
)

func TestNewQueue(t *testing.T) {
	subscriptionID := int64(123)
	messageID := int64(456)

	beforeCreate := time.Now()
	queue := NewQueue(subscriptionID, messageID)
	afterCreate := time.Now()

	// Check IDs
	assert.Equal(t, subscriptionID, queue.SubscriptionID)
	assert.Equal(t, messageID, queue.MessageID)

	// Check status fields
	assert.Equal(t, QueueStatusPending, queue.Status)
	assert.Equal(t, 0, queue.AttemptCount)
	assert.False(t, queue.LastAttemptAt.Valid)
	assert.True(t, queue.NextRetryAt.Valid)
	assert.False(t, queue.LastError.Valid)

	// Check timestamps
	assert.WithinDuration(t, beforeCreate, queue.CreatedAt, 1*time.Second)
	assert.WithinDuration(t, beforeCreate, queue.OperationTimestamp, 1*time.Second)
	assert.WithinDuration(t, beforeCreate.Add(24*time.Hour), queue.ExpiresAt, 1*time.Second)
	assert.WithinDuration(t, beforeCreate, queue.NextRetryAt.Time, 1*time.Second)

	// Check legacy fields
	assert.False(t, queue.IsComplete)
	assert.True(t, queue.RetryAt.Valid)
	assert.False(t, queue.CompletedAt.Valid)

	// Verify timing precision
	assert.True(t, queue.CreatedAt.After(beforeCreate.Add(-1*time.Second)))
	assert.True(t, queue.CreatedAt.Before(afterCreate.Add(1*time.Second)))
}

func TestQueue_MarkFailed(t *testing.T) {
	tests := []struct {
		name             string
		initialAttempts  int
		err              error
		retryAfter       time.Duration
		expectedAttempts int
		expectError      bool
	}{
		{
			name:             "First failure with error",
			initialAttempts:  0,
			err:              errors.New("webhook timeout"),
			retryAfter:       30 * time.Second,
			expectedAttempts: 1,
			expectError:      true,
		},
		{
			name:             "Second failure without error",
			initialAttempts:  1,
			err:              nil,
			retryAfter:       1 * time.Minute,
			expectedAttempts: 2,
			expectError:      false,
		},
		{
			name:             "Fifth failure (DLQ threshold)",
			initialAttempts:  4,
			err:              errors.New("permanent failure"),
			retryAfter:       5 * time.Minute,
			expectedAttempts: 5,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(1, 1)
			queue.AttemptCount = tt.initialAttempts

			beforeMark := time.Now()
			queue.MarkFailed(tt.err, tt.retryAfter)
			afterMark := time.Now()

			assert.Equal(t, QueueStatusFailed, queue.Status)
			assert.Equal(t, tt.expectedAttempts, queue.AttemptCount)
			assert.True(t, queue.LastAttemptAt.Valid)
			assert.True(t, queue.NextRetryAt.Valid)
			assert.WithinDuration(t, beforeMark, queue.LastAttemptAt.Time, 1*time.Second)
			assert.WithinDuration(t, beforeMark.Add(tt.retryAfter), queue.NextRetryAt.Time, 1*time.Second)

			if tt.expectError {
				assert.True(t, queue.LastError.Valid)
				assert.Equal(t, tt.err.Error(), queue.LastError.String)
			} else {
				assert.False(t, queue.LastError.Valid)
			}

			// Ensure timing is correct
			assert.True(t, queue.LastAttemptAt.Time.After(beforeMark.Add(-1*time.Second)))
			assert.True(t, queue.LastAttemptAt.Time.Before(afterMark.Add(1*time.Second)))
		})
	}
}

func TestQueue_MarkSent(t *testing.T) {
	queue := NewQueue(1, 1)
	queue.AttemptCount = 3 // Had some retries before success

	beforeMark := time.Now()
	queue.MarkSent()
	afterMark := time.Now()

	assert.Equal(t, QueueStatusSent, queue.Status)
	assert.True(t, queue.LastAttemptAt.Valid)
	assert.True(t, queue.IsComplete) // Legacy field
	assert.True(t, queue.CompletedAt.Valid)
	assert.WithinDuration(t, beforeMark, queue.LastAttemptAt.Time, 1*time.Second)
	assert.WithinDuration(t, beforeMark, queue.CompletedAt.Time, 1*time.Second)
	assert.True(t, queue.CompletedAt.Time.After(beforeMark.Add(-1*time.Second)))
	assert.True(t, queue.CompletedAt.Time.Before(afterMark.Add(1*time.Second)))
}

func TestQueue_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "Not expired - future",
			expiresAt: time.Now().Add(1 * time.Hour),
			expected:  false,
		},
		{
			name:      "Expired - past",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
		{
			name:      "Just expired",
			expiresAt: time.Now().Add(-1 * time.Second),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(1, 1)
			queue.ExpiresAt = tt.expiresAt

			assert.Equal(t, tt.expected, queue.IsExpired())
		})
	}
}

func TestQueue_ShouldRetry(t *testing.T) {
	tests := []struct {
		name        string
		status      QueueStatus
		nextRetryAt sql.NullTime
		expected    bool
	}{
		{
			name:        "Failed status, retry time passed",
			status:      QueueStatusFailed,
			nextRetryAt: sql.NullTime{Time: time.Now().Add(-1 * time.Minute), Valid: true},
			expected:    true,
		},
		{
			name:        "Failed status, retry time in future",
			status:      QueueStatusFailed,
			nextRetryAt: sql.NullTime{Time: time.Now().Add(1 * time.Minute), Valid: true},
			expected:    false,
		},
		{
			name:        "Pending status (not failed)",
			status:      QueueStatusPending,
			nextRetryAt: sql.NullTime{Time: time.Now().Add(-1 * time.Minute), Valid: true},
			expected:    false,
		},
		{
			name:        "Sent status (not failed)",
			status:      QueueStatusSent,
			nextRetryAt: sql.NullTime{Time: time.Now().Add(-1 * time.Minute), Valid: true},
			expected:    false,
		},
		{
			name:        "Failed but no retry scheduled",
			status:      QueueStatusFailed,
			nextRetryAt: sql.NullTime{},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(1, 1)
			queue.Status = tt.status
			queue.NextRetryAt = tt.nextRetryAt

			assert.Equal(t, tt.expected, queue.ShouldRetry())
		})
	}
}

func TestQueue_CanAttemptDelivery(t *testing.T) {
	tests := []struct {
		name         string
		setupQueue   func(*Queue)
		maxAttempts  int
		expectedErr  error
		errorMessage string
	}{
		{
			name: "Can deliver - pending status",
			setupQueue: func(q *Queue) {
				q.Status = QueueStatusPending
				q.AttemptCount = 0
				q.ExpiresAt = time.Now().Add(1 * time.Hour)
			},
			maxAttempts: 10,
			expectedErr: nil,
		},
		{
			name: "Can retry - failed but ready",
			setupQueue: func(q *Queue) {
				q.Status = QueueStatusFailed
				q.AttemptCount = 2
				q.NextRetryAt = sql.NullTime{Time: time.Now().Add(-1 * time.Minute), Valid: true}
				q.ExpiresAt = time.Now().Add(1 * time.Hour)
			},
			maxAttempts: 10,
			expectedErr: nil,
		},
		{
			name: "Expired item",
			setupQueue: func(q *Queue) {
				q.Status = QueueStatusPending
				q.ExpiresAt = time.Now().Add(-1 * time.Hour)
			},
			maxAttempts:  10,
			expectedErr:  ErrQueueItemExpired,
			errorMessage: "Queue item has expired",
		},
		{
			name: "Already sent",
			setupQueue: func(q *Queue) {
				q.Status = QueueStatusSent
				q.ExpiresAt = time.Now().Add(1 * time.Hour)
			},
			maxAttempts:  10,
			expectedErr:  ErrQueueItemAlreadySent,
			errorMessage: "Queue item already sent",
		},
		{
			name: "Max attempts exceeded",
			setupQueue: func(q *Queue) {
				q.Status = QueueStatusFailed
				q.AttemptCount = 10
				q.ExpiresAt = time.Now().Add(1 * time.Hour)
			},
			maxAttempts:  10,
			expectedErr:  ErrMaxAttemptsExceeded,
			errorMessage: "Maximum delivery attempts exceeded",
		},
		{
			name: "Not ready for retry yet",
			setupQueue: func(q *Queue) {
				q.Status = QueueStatusFailed
				q.AttemptCount = 3
				q.NextRetryAt = sql.NullTime{Time: time.Now().Add(5 * time.Minute), Valid: true}
				q.ExpiresAt = time.Now().Add(1 * time.Hour)
			},
			maxAttempts:  10,
			expectedErr:  ErrNotReadyForRetry,
			errorMessage: "Not ready for retry yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(1, 1)
			tt.setupQueue(&queue)

			err := queue.CanAttemptDelivery(tt.maxAttempts)

			if tt.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr, err)
				if tt.errorMessage != "" {
					assert.Equal(t, tt.errorMessage, err.Error())
				}
			}
		})
	}
}

func TestQueue_RecordAttemptStart(t *testing.T) {
	tests := []struct {
		name                string
		initialStatus       QueueStatus
		initialAttemptCount int
	}{
		{
			name:                "First attempt - pending status",
			initialStatus:       QueueStatusPending,
			initialAttemptCount: 0,
		},
		{
			name:                "Retry attempt - failed status",
			initialStatus:       QueueStatusFailed,
			initialAttemptCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(1, 1)
			queue.Status = tt.initialStatus
			queue.AttemptCount = tt.initialAttemptCount

			beforeRecord := time.Now()
			queue.RecordAttemptStart()
			afterRecord := time.Now()

			assert.True(t, queue.LastAttemptAt.Valid)
			assert.Equal(t, tt.initialAttemptCount, queue.AttemptCount) // Should not change
			assert.WithinDuration(t, beforeRecord, queue.LastAttemptAt.Time, 1*time.Second)
			assert.True(t, queue.LastAttemptAt.Time.After(beforeRecord.Add(-1*time.Second)))
			assert.True(t, queue.LastAttemptAt.Time.Before(afterRecord.Add(1*time.Second)))
		})
	}
}

func TestQueue_ShouldMoveToDLQ(t *testing.T) {
	tests := []struct {
		name         string
		status       QueueStatus
		attemptCount int
		dlqThreshold int
		expected     bool
	}{
		{
			name:         "Should move - threshold reached with failed status",
			status:       QueueStatusFailed,
			attemptCount: 5,
			dlqThreshold: 5,
			expected:     true,
		},
		{
			name:         "Should move - exceeded threshold",
			status:       QueueStatusFailed,
			attemptCount: 7,
			dlqThreshold: 5,
			expected:     true,
		},
		{
			name:         "Should not move - below threshold",
			status:       QueueStatusFailed,
			attemptCount: 4,
			dlqThreshold: 5,
			expected:     false,
		},
		{
			name:         "Should not move - pending status",
			status:       QueueStatusPending,
			attemptCount: 6,
			dlqThreshold: 5,
			expected:     false,
		},
		{
			name:         "Should not move - sent status",
			status:       QueueStatusSent,
			attemptCount: 6,
			dlqThreshold: 5,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(1, 1)
			queue.Status = tt.status
			queue.AttemptCount = tt.attemptCount

			assert.Equal(t, tt.expected, queue.ShouldMoveToDLQ(tt.dlqThreshold))
		})
	}
}

func TestQueue_GetTimeUntilRetry(t *testing.T) {
	tests := []struct {
		name          string
		nextRetryAt   sql.NullTime
		expectedError error
		checkDuration bool
	}{
		{
			name:          "No retry scheduled",
			nextRetryAt:   sql.NullTime{},
			expectedError: ErrNoRetryScheduled,
			checkDuration: false,
		},
		{
			name:          "Retry in future",
			nextRetryAt:   sql.NullTime{Time: time.Now().Add(5 * time.Minute), Valid: true},
			expectedError: nil,
			checkDuration: true,
		},
		{
			name:          "Retry time passed - ready now",
			nextRetryAt:   sql.NullTime{Time: time.Now().Add(-1 * time.Minute), Valid: true},
			expectedError: nil,
			checkDuration: false, // Should return 0 when ready
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(1, 1)
			queue.NextRetryAt = tt.nextRetryAt

			duration, err := queue.GetTimeUntilRetry()

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
				if tt.checkDuration {
					// Should be approximately 5 minutes
					assert.Greater(t, duration, 4*time.Minute)
					assert.Less(t, duration, 6*time.Minute)
				} else {
					assert.Equal(t, time.Duration(0), duration)
				}
			}
		})
	}
}

func TestQueue_GetAge(t *testing.T) {
	queue := NewQueue(1, 1)
	queue.CreatedAt = time.Now().Add(-2 * time.Hour)

	age := queue.GetAge()

	assert.Greater(t, age, 1*time.Hour+55*time.Minute)
	assert.Less(t, age, 2*time.Hour+5*time.Minute)
}

func TestQueue_GetTimeUntilExpiry(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expectPos bool
	}{
		{
			name:      "Expires in future",
			expiresAt: time.Now().Add(3 * time.Hour),
			expectPos: true,
		},
		{
			name:      "Already expired",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expectPos: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(1, 1)
			queue.ExpiresAt = tt.expiresAt

			duration := queue.GetTimeUntilExpiry()

			if tt.expectPos {
				assert.Greater(t, duration, time.Duration(0))
			} else {
				assert.Less(t, duration, time.Duration(0))
			}
		})
	}
}

func TestQueue_DomainErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      DomainError
		code     string
		message  string
		errorStr string
	}{
		{
			name:     "QueueItemExpired",
			err:      ErrQueueItemExpired,
			code:     "QUEUE_EXPIRED",
			message:  "Queue item has expired",
			errorStr: "Queue item has expired",
		},
		{
			name:     "QueueItemAlreadySent",
			err:      ErrQueueItemAlreadySent,
			code:     "ALREADY_SENT",
			message:  "Queue item already sent",
			errorStr: "Queue item already sent",
		},
		{
			name:     "MaxAttemptsExceeded",
			err:      ErrMaxAttemptsExceeded,
			code:     "MAX_ATTEMPTS",
			message:  "Maximum delivery attempts exceeded",
			errorStr: "Maximum delivery attempts exceeded",
		},
		{
			name:     "NotReadyForRetry",
			err:      ErrNotReadyForRetry,
			code:     "NOT_READY",
			message:  "Not ready for retry yet",
			errorStr: "Not ready for retry yet",
		},
		{
			name:     "NoRetryScheduled",
			err:      ErrNoRetryScheduled,
			code:     "NO_RETRY",
			message:  "No retry scheduled",
			errorStr: "No retry scheduled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.message, tt.err.Message)
			assert.Equal(t, tt.errorStr, tt.err.Error())
		})
	}
}

// Integration test - full lifecycle simulation.
func TestQueue_FullLifecycle(t *testing.T) {
	t.Run("Successful delivery after retries", func(t *testing.T) {
		queue := NewQueue(123, 456)

		// Initial state
		assert.Equal(t, QueueStatusPending, queue.Status)
		assert.Equal(t, 0, queue.AttemptCount)

		// First attempt - fail
		err := queue.CanAttemptDelivery(10)
		assert.NoError(t, err)
		queue.MarkFailed(errors.New("timeout"), 30*time.Second)
		assert.Equal(t, QueueStatusFailed, queue.Status)
		assert.Equal(t, 1, queue.AttemptCount)

		// Wait for retry (simulate)
		queue.NextRetryAt = sql.NullTime{Time: time.Now().Add(-1 * time.Second), Valid: true}
		assert.True(t, queue.ShouldRetry())

		// Second attempt - fail
		err = queue.CanAttemptDelivery(10)
		assert.NoError(t, err)
		queue.MarkFailed(errors.New("connection refused"), 1*time.Minute)
		assert.Equal(t, QueueStatusFailed, queue.Status)
		assert.Equal(t, 2, queue.AttemptCount)

		// Third attempt - success
		queue.NextRetryAt = sql.NullTime{Time: time.Now().Add(-1 * time.Second), Valid: true}
		err = queue.CanAttemptDelivery(10)
		assert.NoError(t, err)
		queue.MarkSent()
		assert.Equal(t, QueueStatusSent, queue.Status)
		assert.True(t, queue.IsComplete)
		assert.Equal(t, 2, queue.AttemptCount) // Still 2, success doesn't increment
	})

	t.Run("Move to DLQ after threshold", func(t *testing.T) {
		queue := NewQueue(123, 456)
		dlqThreshold := 5

		// Simulate 5 failed attempts
		for i := 0; i < dlqThreshold; i++ {
			queue.MarkFailed(errors.New("persistent error"), 1*time.Minute)
			queue.NextRetryAt = sql.NullTime{Time: time.Now().Add(-1 * time.Second), Valid: true} // Ready for retry
		}

		assert.Equal(t, dlqThreshold, queue.AttemptCount)
		assert.True(t, queue.ShouldMoveToDLQ(dlqThreshold))
	})

	t.Run("Expired before delivery", func(t *testing.T) {
		queue := NewQueue(123, 456)
		queue.ExpiresAt = time.Now().Add(-1 * time.Hour)

		err := queue.CanAttemptDelivery(10)
		assert.Error(t, err)
		assert.Equal(t, ErrQueueItemExpired, err)
	})
}
