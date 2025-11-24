package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPublisher_TableName(t *testing.T) {
	pub := Publisher{}
	assert.Equal(t, "pubsub_publisher", pub.TableName())
}

func TestNewPublisher(t *testing.T) {
	code := "service-a"
	name := "Service A"
	description := "Main API service"

	pub := NewPublisher(code, name, description)

	assert.Equal(t, int64(0), pub.ID)
	assert.Equal(t, code, pub.Code)
	assert.Equal(t, name, pub.Name)
	assert.Equal(t, description, pub.Description)
	assert.WithinDuration(t, time.Now(), pub.CreatedAt, time.Second)
}
