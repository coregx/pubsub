package model

import (
	"database/sql"
	"time"
)

// QueueWithDetails - Extended queue model with new TTL-based retry fields (Migration 00018).
// Used by NotificationService for guaranteed delivery with exponential backoff.
type QueueWithDetails struct {
	// Core fields
	QueueID        int64  `json:"queueId"`
	MessageID      int64  `json:"messageID"`
	SubscriptionID int64  `json:"subscriptionID"`
	Status         string `json:"status"` // "pending", "sent", "failed"

	// Retry tracking
	AttemptCount  int        `json:"attemptCount"`
	LastAttemptAt *time.Time `json:"lastAttemptAt"`
	NextRetryAt   *time.Time `json:"nextRetryAt"`
	LastError     string     `json:"lastError"`

	// Ordering and TTL
	SequenceNumber int64      `json:"sequenceNumber"`
	OperationTime  time.Time  `json:"operationTime"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	CreatedAt      time.Time  `json:"createdAt"`

	// Message details
	Identifier string `json:"identifier"`
	Data       []byte `json:"data"`

	// Subscription details
	CallbackURL  string `json:"callbackURL"`
	SubscriberID int64  `json:"subscriberID"`
}

// SubscriptionV2 - Extended with last_notification_at for duplicate prevention.
type SubscriptionV2 struct {
	ID                 int64      `json:"id"`
	SubscriberID       int64      `json:"subscriberID"`
	TopicID            int64      `json:"topicID"`
	Identifier         string     `json:"identifier"`
	CallbackURL        string     `json:"callbackURL"`
	IsActive           bool       `json:"isActive"`
	TrackID            string     `json:"trackId"`
	CreatedAt          time.Time  `json:"createdAt"`
	LastNotificationAt *time.Time `json:"lastNotificationAt"` // New field (Migration 00018)
}

// NotificationLog - Log of all notification attempts for analytics and debugging.
type NotificationLog struct {
	ID             int64          `json:"id"`
	SubscriptionID int64          `json:"subscriptionID"`
	MessageID      int64          `json:"messageID"`
	Identifier     string         `json:"identifier"`
	TopicCode      string         `json:"topicCode"`
	SubscriberType string         `json:"subscriberType"` // "client", "service"
	SubscriberID   int64          `json:"subscriberID"`
	DeliveryMethod string         `json:"deliveryMethod"` // "webhook", "grpc", "pull"
	Status         string         `json:"status"`         // "pending", "sent", "failed", "skipped"
	SkippedReason  sql.NullString `json:"skippedReason"`
	SentAt         sql.NullTime   `json:"sentAt"`
	CreatedAt      time.Time      `json:"createdAt"`
}

// TableName returns the database table name for NotificationLog.
func (t NotificationLog) TableName() string {
	return tablePrefix + "notification_log"
}

// NewNotificationLog creates a new notification log entry.
func NewNotificationLog(
	subscriptionID int64,
	messageID int64,
	identifier string,
	topicCode string,
	subscriberType string,
	subscriberID int64,
	deliveryMethod string,
	status string,
) NotificationLog {
	return NotificationLog{
		SubscriptionID: subscriptionID,
		MessageID:      messageID,
		Identifier:     identifier,
		TopicCode:      topicCode,
		SubscriberType: subscriberType,
		SubscriberID:   subscriberID,
		DeliveryMethod: deliveryMethod,
		Status:         status,
		CreatedAt:      time.Now(),
	}
}
