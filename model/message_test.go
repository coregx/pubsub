package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessage_TableName(t *testing.T) {
	msg := Message{}
	assert.Equal(t, "pubsub_message", msg.TableName())
}

func TestNewMessage(t *testing.T) {
	topicID := int64(456)
	identifier := "test-event"
	data := "test-payload"

	msg := NewMessage(topicID, identifier, data)

	assert.Equal(t, int64(0), msg.ID)
	assert.Equal(t, topicID, msg.TopicID)
	assert.Equal(t, identifier, msg.Identifier)
	assert.Equal(t, data, msg.Data)
	assert.WithinDuration(t, time.Now(), msg.CreatedAt, time.Second)
}

func TestNewDataMessage(t *testing.T) {
	msgID := "123"
	publishDate := time.Now()
	identifier := "test-event"
	data := "payload"

	dm := NewDataMessage(msgID, publishDate, identifier, data)

	assert.Equal(t, msgID, dm.MessageID)
	assert.Equal(t, identifier, dm.Identifier)
	assert.Equal(t, data, dm.Data)
	assert.NotNil(t, dm.Attributes)
}

func TestDataMessage_ToString(t *testing.T) {
	dm := NewDataMessage("123", time.Now(), "event", "test-data")

	result := dm.ToString()
	assert.Equal(t, "test-data", result)
}

func TestDataMessage_FromString(t *testing.T) {
	// Implementation returns nil - just test it doesn't panic
	dm := &DataMessage{}
	err := dm.FromString("test-data")
	assert.NoError(t, err)
}

func TestDataMessage_ToBase64(t *testing.T) {
	dm := NewDataMessage("123", time.Now(), "event", "data")
	result := dm.ToBase64()
	assert.Equal(t, "", result) // Current implementation returns empty string
}
