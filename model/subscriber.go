package model

import "time"

// Subscriber represents a message consumer in the pub/sub system.
// Subscribers receive messages via webhooks when topics they're subscribed to receive new messages.
//
// Each subscriber:
//   - Has a webhook URL for message delivery
//   - Is associated with a client (tenant/organization)
//   - Can have multiple subscriptions to different topics
//   - Can be activated/deactivated
type Subscriber struct {
	ID         int64     `json:"id"`                          // Unique subscriber ID
	ClientID   int64     `json:"clientID" db:"client_id"`     // Associated client/tenant ID
	Name       string    `json:"name"`                        // Subscriber name
	WebhookURL string    `json:"webhookURL" db:"webhook_url"` // HTTP endpoint for message delivery
	IsActive   bool      `json:"isActive" db:"is_active"`     // Only active subscribers receive messages
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`   // Subscriber registration time
}

// TableName returns the database table name for Subscriber.
func (t Subscriber) TableName() string {
	return tablePrefix + "subscriber"
}

// NewSubscriber creates a new active subscriber.
//
// Parameters:
//   - clientID: The client/tenant this subscriber belongs to
//   - name: Human-readable subscriber name
//   - webhookURL: HTTP endpoint to receive message deliveries
func NewSubscriber(clientID int64, name, webhookURL string) Subscriber {
	return Subscriber{
		ID:         0,
		ClientID:   clientID,
		Name:       name,
		WebhookURL: webhookURL,
		IsActive:   true,
		CreatedAt:  time.Now(),
	}
}
