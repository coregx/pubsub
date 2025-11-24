package model

import (
	"database/sql"
	"time"
)

// QueueStatus represents the lifecycle state of a queue item.
type QueueStatus string

const (
	// QueueStatusPending indicates the message is awaiting first delivery attempt.
	QueueStatusPending QueueStatus = "pending"

	// QueueStatusSent indicates successful message delivery.
	QueueStatusSent QueueStatus = "sent"

	// QueueStatusFailed indicates delivery failed and item is awaiting retry.
	QueueStatusFailed QueueStatus = "failed"
)

// Queue represents a message queued for delivery to a subscriber.
// It contains retry logic state, timing information, and error tracking.
//
// Queue items follow this lifecycle:
//  1. Created with status=PENDING
//  2. Delivery attempted → either SENT (success) or FAILED (retry)
//  3. FAILED items retry with exponential backoff
//  4. After exceeding retry threshold → moved to Dead Letter Queue (DLQ)
//
// Business logic methods:
//   - MarkSent/MarkFailed: Update status after delivery attempt
//   - CanAttemptDelivery: Check if delivery can be attempted
//   - ShouldRetry: Check if item is ready for retry
//   - ShouldMoveToDLQ: Check if exhausted retries
//
// This model implements Domain-Driven Design with rich business logic.
type Queue struct {
	ID                 int64          `json:"id"`
	SubscriptionID     int64          `json:"subscriptionID"`
	MessageID          int64          `json:"messageID"`
	Status             QueueStatus    `json:"status" db:"status"`                          // NEW: from 00019
	AttemptCount       int            `json:"attemptCount" db:"attempt_count"`             // NEW: from 00019
	LastAttemptAt      sql.NullTime   `json:"lastAttemptAt" db:"last_attempt_at"`          // NEW: from 00019
	NextRetryAt        sql.NullTime   `json:"nextRetryAt" db:"next_retry_at"`              // NEW: from 00019
	LastError          sql.NullString `json:"lastError" db:"last_error"`                   // NEW: from 00019
	ExpiresAt          time.Time      `json:"expiresAt" db:"expires_at"`                   // NEW: from 00019
	SequenceNumber     int64          `json:"sequenceNumber" db:"sequence_number"`         // NEW: from 00019
	OperationTimestamp time.Time      `json:"operationTimestamp" db:"operation_timestamp"` // NEW: from 00019
	RetryAt            sql.NullTime   `json:"retryAt"`                                     // LEGACY: keep for backward compatibility
	IsComplete         bool           `json:"isComplete"`                                  // LEGACY: deprecated, use Status
	CompletedAt        sql.NullTime   `json:"completedAt"`
	CreatedAt          time.Time      `json:"createdAt"`
}

// TableName returns the database table name for Queue.
func (t *Queue) TableName() string {
	return tablePrefix + "queue"
}

// NewQueue creates a new queue item for message delivery.
// Initial state: PENDING, AttemptCount=0, NextRetryAt=now (ready immediately).
// Default expiry: 24 hours from creation.
func NewQueue(subscriptionID, messageID int64) Queue {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour) // Default 24h expiry

	return Queue{
		ID:                 0,
		SubscriptionID:     subscriptionID,
		MessageID:          messageID,
		Status:             QueueStatusPending,
		AttemptCount:       0,
		LastAttemptAt:      sql.NullTime{},
		NextRetryAt:        sql.NullTime{Time: now, Valid: true}, // Ready to send immediately
		LastError:          sql.NullString{},
		ExpiresAt:          expiresAt,
		SequenceNumber:     0, // Will be set by repository
		OperationTimestamp: now,
		RetryAt:            sql.NullTime{Time: now, Valid: true}, // LEGACY
		IsComplete:         false,                                // LEGACY
		CompletedAt:        sql.NullTime{},
		CreatedAt:          now,
	}
}

// SetComplete marks the queue item as complete (deprecated, use MarkSent instead).
func (t *Queue) SetComplete() {
	t.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	t.IsComplete = true
	t.Status = QueueStatusSent
}

// MarkFailed marks the queue item as failed and schedules the next retry attempt.
// Increments attempt count, records error message, and calculates next retry time.
//
// Parameters:
//   - err: The delivery error (stored in LastError)
//   - retryAfter: Duration to wait before next retry (exponential backoff)
func (t *Queue) MarkFailed(err error, retryAfter time.Duration) {
	now := time.Now()
	t.Status = QueueStatusFailed
	t.AttemptCount++
	t.LastAttemptAt = sql.NullTime{Time: now, Valid: true}
	t.NextRetryAt = sql.NullTime{Time: now.Add(retryAfter), Valid: true}
	if err != nil {
		t.LastError = sql.NullString{String: err.Error(), Valid: true}
	}
}

// MarkSent marks the queue item as successfully delivered.
// Sets status to SENT and updates timing fields.
func (t *Queue) MarkSent() {
	now := time.Now()
	t.Status = QueueStatusSent
	t.LastAttemptAt = sql.NullTime{Time: now, Valid: true}
	t.SetComplete() // Also set legacy fields
}

// IsExpired checks if the queue item has passed its expiration time.
// Expired items are cleaned up by the queue worker.
func (t *Queue) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// ShouldRetry checks if the item is ready for retry attempt.
// Returns true if status=FAILED, has valid NextRetryAt, and time has passed.
func (t *Queue) ShouldRetry() bool {
	if t.Status != QueueStatusFailed {
		return false
	}
	if !t.NextRetryAt.Valid {
		return false
	}
	return time.Now().After(t.NextRetryAt.Time)
}

// CanAttemptDelivery validates whether delivery can be attempted based on business rules.
// Checks expiration, status, max attempts, and retry timing.
//
// Returns error if delivery cannot be attempted:
//   - ErrQueueItemExpired: Item has expired
//   - ErrQueueItemAlreadySent: Already successfully delivered
//   - ErrMaxAttemptsExceeded: Exceeded retry limit
//   - ErrNotReadyForRetry: Too soon for retry
func (t *Queue) CanAttemptDelivery(maxAttempts int) error {
	if t.IsExpired() {
		return ErrQueueItemExpired
	}
	if t.Status == QueueStatusSent {
		return ErrQueueItemAlreadySent
	}
	if t.AttemptCount >= maxAttempts {
		return ErrMaxAttemptsExceeded
	}
	if t.Status == QueueStatusFailed && !t.ShouldRetry() {
		return ErrNotReadyForRetry
	}
	return nil
}

// RecordAttemptStart marks the beginning of a delivery attempt.
// Records timing only - attempt count is incremented by MarkFailed or MarkSent.
func (t *Queue) RecordAttemptStart() {
	t.LastAttemptAt = sql.NullTime{Time: time.Now(), Valid: true}
	// AttemptCount will be incremented by MarkFailed or MarkSent
	// This method only records timing
}

// ShouldMoveToDLQ checks if the item should be moved to the Dead Letter Queue.
// Returns true when attempt count reaches the DLQ threshold and status is FAILED.
func (t *Queue) ShouldMoveToDLQ(dlqThreshold int) bool {
	return t.AttemptCount >= dlqThreshold && t.Status == QueueStatusFailed
}

// GetTimeUntilRetry returns the duration until the next retry attempt.
// Returns 0 if ready for retry now, or error if no retry is scheduled.
func (t *Queue) GetTimeUntilRetry() (time.Duration, error) {
	if !t.NextRetryAt.Valid {
		return 0, ErrNoRetryScheduled
	}
	duration := time.Until(t.NextRetryAt.Time)
	if duration < 0 {
		return 0, nil // Ready for retry now
	}
	return duration, nil
}

// GetAge returns how long the queue item has existed since creation.
func (t *Queue) GetAge() time.Duration {
	return time.Since(t.CreatedAt)
}

// GetTimeUntilExpiry returns the duration until the item expires.
// Negative duration means already expired.
func (t *Queue) GetTimeUntilExpiry() time.Duration {
	return time.Until(t.ExpiresAt)
}

// Domain errors returned by Queue business logic methods.
var (
	// ErrQueueItemExpired indicates the queue item has passed its expiration time.
	ErrQueueItemExpired = DomainError{Code: "QUEUE_EXPIRED", Message: "Queue item has expired"}

	// ErrQueueItemAlreadySent indicates the message was already successfully delivered.
	ErrQueueItemAlreadySent = DomainError{Code: "ALREADY_SENT", Message: "Queue item already sent"}

	// ErrMaxAttemptsExceeded indicates the item has reached the maximum retry attempts.
	ErrMaxAttemptsExceeded = DomainError{Code: "MAX_ATTEMPTS", Message: "Maximum delivery attempts exceeded"}

	// ErrNotReadyForRetry indicates the retry delay hasn't elapsed yet.
	ErrNotReadyForRetry = DomainError{Code: "NOT_READY", Message: "Not ready for retry yet"}

	// ErrNoRetryScheduled indicates no retry time has been set for this item.
	ErrNoRetryScheduled = DomainError{Code: "NO_RETRY", Message: "No retry scheduled"}
)

// DomainError represents a domain-level business rule violation.
// Used by Queue methods to return business logic errors.
type DomainError struct {
	Code    string // Error code for programmatic handling
	Message string // Human-readable error message
}

func (e DomainError) Error() string {
	return e.Message
}
