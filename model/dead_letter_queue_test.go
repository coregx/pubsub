package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeadLetterQueue_TableName(t *testing.T) {
	dlq := DeadLetterQueue{}
	assert.Equal(t, "pubsub_dlq", dlq.TableName())
}

func TestNewDeadLetterQueue(t *testing.T) {
	subscriptionID := int64(100)
	messageID := int64(200)
	originalQueueID := int64(300)
	attemptCount := 5
	lastError := "connection timeout"
	failureReason := "Max retry attempts exceeded"
	firstAttempt := time.Now().Add(-1 * time.Hour)
	lastAttempt := time.Now().Add(-5 * time.Minute)
	messageData := `{"event":"test"}`
	callbackURL := "https://example.com/webhook"

	dlq := NewDeadLetterQueue(
		subscriptionID,
		messageID,
		originalQueueID,
		attemptCount,
		lastError,
		failureReason,
		firstAttempt,
		lastAttempt,
		messageData,
		callbackURL,
	)

	assert.Equal(t, int64(0), dlq.ID)
	assert.Equal(t, subscriptionID, dlq.SubscriptionID)
	assert.Equal(t, messageID, dlq.MessageID)
	assert.Equal(t, originalQueueID, dlq.OriginalQueueID)
	assert.Equal(t, attemptCount, dlq.AttemptCount)
	assert.Equal(t, lastError, dlq.LastError)
	assert.Equal(t, failureReason, dlq.FailureReason)
	assert.Equal(t, firstAttempt, dlq.FirstAttemptAt)
	assert.Equal(t, lastAttempt, dlq.LastAttemptAt)
	assert.WithinDuration(t, time.Now(), dlq.MovedToDLQAt, time.Second)
	assert.Equal(t, messageData, dlq.MessageData)
	assert.Equal(t, callbackURL, dlq.CallbackURL)
	assert.False(t, dlq.IsResolved)
	assert.Nil(t, dlq.ResolvedAt)
	assert.Empty(t, dlq.ResolvedBy)
	assert.Empty(t, dlq.ResolutionNote)
	assert.WithinDuration(t, time.Now(), dlq.CreatedAt, time.Second)
}

func TestDeadLetterQueue_Resolve(t *testing.T) {
	dlq := NewDeadLetterQueue(
		100, 200, 300, 5,
		"error", "max retries",
		time.Now(), time.Now(),
		"data", "url",
	)

	assert.False(t, dlq.IsResolved)
	assert.Nil(t, dlq.ResolvedAt)

	resolvedBy := "admin@example.com"
	note := "Fixed webhook endpoint"

	dlq.Resolve(resolvedBy, note)

	assert.True(t, dlq.IsResolved)
	assert.NotNil(t, dlq.ResolvedAt)
	assert.WithinDuration(t, time.Now(), *dlq.ResolvedAt, time.Second)
	assert.Equal(t, resolvedBy, dlq.ResolvedBy)
	assert.Equal(t, note, dlq.ResolutionNote)
}

func TestDeadLetterQueue_GetAge(t *testing.T) {
	dlq := NewDeadLetterQueue(
		100, 200, 300, 5,
		"error", "max retries",
		time.Now(), time.Now(),
		"data", "url",
	)

	// Just created, age should be near zero
	age := dlq.GetAge()
	assert.True(t, age >= 0)
	assert.True(t, age < time.Second)
}

func TestDeadLetterQueue_IsOld(t *testing.T) {
	dlq := NewDeadLetterQueue(
		100, 200, 300, 5,
		"error", "max retries",
		time.Now(), time.Now(),
		"data", "url",
	)

	// Just created - should not be old
	assert.False(t, dlq.IsOld(1*time.Hour))

	// Simulate old item by setting MovedToDLQAt in the past
	dlq.MovedToDLQAt = time.Now().Add(-2 * time.Hour)
	assert.True(t, dlq.IsOld(1*time.Hour))
	assert.False(t, dlq.IsOld(3*time.Hour))
}
