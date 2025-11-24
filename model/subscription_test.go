package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubscription_TableName(t *testing.T) {
	sub := Subscription{}
	assert.Equal(t, "pubsub_subscription", sub.TableName())
}

func TestNewSubscription(t *testing.T) {
	subscriberID := int64(100)
	topicID := int64(200)
	identifier := "user.created"
	callbackURL := "https://example.com/webhook"

	sub := NewSubscription(subscriberID, topicID, identifier, callbackURL)

	assert.Equal(t, int64(0), sub.ID)
	assert.Equal(t, subscriberID, sub.SubscriberID)
	assert.Equal(t, topicID, sub.TopicID)
	assert.Equal(t, identifier, sub.Identifier)
	assert.True(t, sub.IsActive)
	assert.WithinDuration(t, time.Now(), sub.CreatedAt, time.Second)
	assert.False(t, sub.DeletedAt.Valid)
}

func TestSubscription_Deactivate(t *testing.T) {
	sub := NewSubscription(100, 200, "test.event", "https://example.com/webhook")
	assert.True(t, sub.IsActive)
	assert.False(t, sub.DeletedAt.Valid)

	sub.Deactivate()

	assert.False(t, sub.IsActive)
	assert.True(t, sub.DeletedAt.Valid)
	assert.WithinDuration(t, time.Now(), sub.DeletedAt.Time, time.Second)
}

func TestSubscription_DeactivateIdempotent(t *testing.T) {
	sub := NewSubscription(100, 200, "test.event", "https://example.com/webhook")

	// Deactivate first time
	sub.Deactivate()

	time.Sleep(10 * time.Millisecond)

	// Deactivate second time - timestamp will change but that's OK
	sub.Deactivate()

	assert.False(t, sub.IsActive)
	assert.True(t, sub.DeletedAt.Valid)
}

func TestSubscriptionFull_TableName(t *testing.T) {
	sf := SubscriptionFull{}
	assert.Equal(t, "pubsub_subscription", sf.TableName())
}
