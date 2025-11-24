package model

import "time"

// Topic represents a message category/channel in the pub/sub system.
// Topics define the routing mechanism for messages to subscribers.
//
// When a message is published to a topic, it is delivered to all active
// subscriptions matching that topic and identifier.
//
// Topics can be hierarchical using dot notation (e.g., "user.created", "order.payment.completed").
type Topic struct {
	ID          int64     `json:"id"`                        // Unique topic ID
	Code        string    `json:"code" db:"topic_code"`      // Unique topic code (e.g., "user.signup")
	Name        string    `json:"name"`                      // Human-readable topic name
	Description string    `json:"description"`               // Topic purpose and details
	IsActive    bool      `json:"isActive" db:"is_active"`   // Only active topics accept new messages
	CreatedAt   time.Time `json:"createdAt" db:"created_at"` // Topic creation time
}

// TableName returns the database table name for Topic.
func (t Topic) TableName() string {
	return tablePrefix + "topic"
}

// NewTopic creates a new active topic.
//
// Parameters:
//   - code: Unique topic identifier (e.g., "user.signup", "order.created")
//   - name: Human-readable name for display
//   - description: Purpose and usage details for this topic
func NewTopic(code, name, description string) Topic {
	return Topic{
		ID:          0,
		Code:        code,
		Name:        name,
		Description: description,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
}
