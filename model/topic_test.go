package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTopic_TableName(t *testing.T) {
	topic := Topic{}
	assert.Equal(t, "pubsub_topic", topic.TableName())
}

func TestNewTopic(t *testing.T) {
	code := "user.events"
	name := "User Events"
	description := "All user-related events"

	topic := NewTopic(code, name, description)

	assert.Equal(t, int64(0), topic.ID)
	assert.Equal(t, code, topic.Code)
	assert.Equal(t, name, topic.Name)
	assert.Equal(t, description, topic.Description)
	assert.WithinDuration(t, time.Now(), topic.CreatedAt, time.Second)
}
