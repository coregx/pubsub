package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubscriber_TableName(t *testing.T) {
	sub := Subscriber{}
	assert.Equal(t, "pubsub_subscriber", sub.TableName())
}

func TestNewSubscriber(t *testing.T) {
	clientID := int64(123)
	name := "Test Service"
	webhookURL := "https://example.com/webhook"

	sub := NewSubscriber(clientID, name, webhookURL)

	assert.Equal(t, int64(0), sub.ID)
	assert.Equal(t, clientID, sub.ClientID)
	assert.Equal(t, name, sub.Name)
	assert.Equal(t, webhookURL, sub.WebhookURL)
	assert.True(t, sub.IsActive) // Default value
	assert.WithinDuration(t, time.Now(), sub.CreatedAt, time.Second)
}
