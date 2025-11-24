package model

import (
	"database/sql"
	"time"
)

// Subscription represents a subscriber's subscription to a topic.
// Subscriptions connect subscribers to topics, enabling message delivery routing.
//
// Each subscription:
//   - Links a subscriber to a topic
//   - Filters messages by identifier (e.g., "user-123")
//   - Can be activated/deactivated (soft delete)
//   - Creates queue items when matching messages are published
//
// Lifecycle: Active subscriptions receive new messages, inactive ones don't.
type Subscription struct {
	ID           int64        `json:"id"`           // Unique subscription ID
	SubscriberID int64        `json:"subscriberID"` // Subscriber who owns this subscription
	TopicID      int64        `json:"topicID"`      // Topic being subscribed to
	Identifier   string       `json:"identifier"`   // Event identifier filter
	IsActive     bool         `json:"isActive"`     // Active subscriptions receive messages
	CreatedAt    time.Time    `json:"createdAt"`    // Subscription creation time
	DeletedAt    sql.NullTime `json:"deletedAt"`    // Soft delete timestamp
}

// TableName returns the database table name for Subscription.
func (m Subscription) TableName() string {
	return tablePrefix + "subscription"
}

// NewSubscription creates a new active subscription.
// The callbackURL parameter is retained for compatibility but stored on the Subscriber.
//
// Parameters:
//   - subscriberID: The subscriber creating this subscription
//   - topicID: The topic to subscribe to
//   - identifier: Event identifier for filtering (e.g., "user-123", "order-*")
//   - callbackURL: Webhook URL (typically stored on Subscriber, parameter kept for compatibility)
func NewSubscription(subscriberID, topicID int64, identifier, _ string) Subscription {
	return Subscription{
		ID:           0,
		SubscriberID: subscriberID,
		TopicID:      topicID,
		Identifier:   identifier,
		IsActive:     true,
		CreatedAt:    time.Now(),
		DeletedAt:    sql.NullTime{},
	}
}

// Deactivate performs a soft delete on the subscription.
// Deactivated subscriptions stop receiving new messages but are retained for audit purposes.
func (m *Subscription) Deactivate() {
	m.IsActive = false
	m.DeletedAt = sql.NullTime{Time: time.Now(), Valid: true}
}

// SubscriptionFull is an extended subscription view with denormalized fields.
// Used by queries that need subscription details along with statistics and webhook URLs.
type SubscriptionFull struct {
	Subscription        // Embedded base subscription
	Messages     int    // Count of messages delivered
	CallbackURL  string // Denormalized webhook URL from subscriber
}
