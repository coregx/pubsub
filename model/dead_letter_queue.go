package model

import (
	"time"
)

// DeadLetterQueue represents a permanently failed message that exceeded the retry threshold.
// Messages are automatically moved here after exhausting all retry attempts (typically 5-7 attempts).
//
// The DLQ serves as:
//   - Failure audit log with full diagnostic information
//   - Manual intervention queue for operations teams
//   - Source for failure analysis and monitoring
//
// Business logic methods:
//   - Resolve: Mark item as manually resolved
//   - GetAge: Calculate time in DLQ
//   - IsOld: Check if item needs attention
//
// Items remain in DLQ until manually resolved or deleted.
type DeadLetterQueue struct {
	ID              int64 `json:"id"`
	SubscriptionID  int64 `json:"subscriptionID" db:"subscription_id"`
	MessageID       int64 `json:"messageID" db:"message_id"`
	OriginalQueueID int64 `json:"originalQueueId" db:"original_queue_id"` // Reference to original queue item

	// Failure information
	AttemptCount  int    `json:"attemptCount" db:"attempt_count"`   // Total attempts before DLQ
	LastError     string `json:"lastError" db:"last_error"`         // Last error message
	FailureReason string `json:"failureReason" db:"failure_reason"` // Reason for moving to DLQ

	// Timing information
	FirstAttemptAt time.Time `json:"firstAttemptAt" db:"first_attempt_at"` // When first delivery was attempted
	LastAttemptAt  time.Time `json:"lastAttemptAt" db:"last_attempt_at"`   // When last attempt failed
	MovedToDLQAt   time.Time `json:"movedToDlqAt" db:"moved_to_dlq_at"`    // When moved to DLQ

	// Message data (denormalized for easy access)
	MessageData string `json:"messageData" db:"message_data"` // Original message payload
	CallbackURL string `json:"callbackURL" db:"callback_url"` // Target webhook URL

	// Lifecycle
	IsResolved     bool       `json:"isResolved" db:"is_resolved"`         // Manual resolution flag
	ResolvedAt     *time.Time `json:"resolvedAt" db:"resolved_at"`         // When manually resolved
	ResolvedBy     string     `json:"resolvedBy" db:"resolved_by"`         // Who resolved (user/system)
	ResolutionNote string     `json:"resolutionNote" db:"resolution_note"` // Resolution explanation

	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// TableName returns the database table name for DeadLetterQueue.
func (d DeadLetterQueue) TableName() string {
	return tablePrefix + "dlq"
}

// NewDeadLetterQueue creates a new Dead Letter Queue entry from a failed queue item.
// This is called automatically by QueueWorker when a queue item exceeds the retry threshold.
//
// Denormalizes message data and callback URL for easy access without joining tables.
func NewDeadLetterQueue(
	subscriptionID, messageID, originalQueueID int64,
	attemptCount int,
	lastError, failureReason string,
	firstAttemptAt, lastAttemptAt time.Time,
	messageData, callbackURL string,
) DeadLetterQueue {
	return DeadLetterQueue{
		ID:              0,
		SubscriptionID:  subscriptionID,
		MessageID:       messageID,
		OriginalQueueID: originalQueueID,
		AttemptCount:    attemptCount,
		LastError:       lastError,
		FailureReason:   failureReason,
		FirstAttemptAt:  firstAttemptAt,
		LastAttemptAt:   lastAttemptAt,
		MovedToDLQAt:    time.Now(),
		MessageData:     messageData,
		CallbackURL:     callbackURL,
		IsResolved:      false,
		ResolvedAt:      nil,
		ResolvedBy:      "",
		ResolutionNote:  "",
		CreatedAt:       time.Now(),
	}
}

// Resolve marks the DLQ item as manually resolved by an operator.
// This is typically used after:
//   - Manual message replay
//   - Fixing the root cause and redelivering
//   - Determining the failure is acceptable and can be ignored
//
// Parameters:
//   - resolvedBy: Username/system that resolved the item
//   - note: Explanation of the resolution action taken
func (d *DeadLetterQueue) Resolve(resolvedBy, note string) {
	now := time.Now()
	d.IsResolved = true
	d.ResolvedAt = &now
	d.ResolvedBy = resolvedBy
	d.ResolutionNote = note
}

// GetAge returns how long the item has been in the Dead Letter Queue.
func (d *DeadLetterQueue) GetAge() time.Duration {
	return time.Since(d.MovedToDLQAt)
}

// IsOld checks if the item has been in DLQ longer than the threshold duration.
// Used to identify stuck items that need urgent attention.
func (d *DeadLetterQueue) IsOld(threshold time.Duration) bool {
	return d.GetAge() > threshold
}

// DLQStats represents aggregate statistics for the Dead Letter Queue.
// Used for monitoring dashboards and operational visibility.
type DLQStats struct {
	TotalItems       int       `json:"totalItems"`
	UnresolvedItems  int       `json:"unresolvedItems"`
	ResolvedItems    int       `json:"resolvedItems"`
	OldestItemAge    int64     `json:"oldestItemAge"` // Seconds
	NewestItemAge    int64     `json:"newestItemAge"` // Seconds
	TopFailureReason string    `json:"topFailureReason"`
	LastUpdated      time.Time `json:"lastUpdated"`
}
