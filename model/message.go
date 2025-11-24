package model

import "time"

// Message represents a published message in the pub/sub system.
// Messages are immutable once created and contain the actual payload to be delivered.
//
// Each published message creates queue items for all active subscriptions to its topic.
// Messages are retained for archival/audit purposes even after successful delivery.
type Message struct {
	ID         int64     `json:"id"`         // Unique message ID
	TopicID    int64     `json:"topicID"`    // Topic this message belongs to
	Identifier string    `json:"identifier"` // Event identifier (e.g., "user-123")
	Data       string    `json:"data"`       // Message payload (JSON or string)
	CreatedAt  time.Time `json:"createdAt"`  // Publication timestamp
}

// TableName returns the database table name for Message.
func (t Message) TableName() string {
	return tablePrefix + "message"
}

// NewMessage creates a new message for publication.
// Messages are immutable after creation.
//
// Parameters:
//   - topicID: The topic to publish to
//   - identifier: Event identifier for filtering/routing
//   - data: Message payload (typically JSON)
func NewMessage(topicID int64, identifier, data string) Message {
	return Message{
		ID:         0,
		TopicID:    topicID,
		Identifier: identifier,
		Data:       data,
		CreatedAt:  time.Now(),
	}
}

// PublishResult represents the result of a publish operation.
// Currently contains a simple success flag, can be extended with message IDs, etc.
type PublishResult struct {
	Result bool // True if publish succeeded
}
