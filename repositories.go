package pubsub

import (
	"context"
	"time"

	"github.com/coregx/pubsub/model"
)

// Filter represents query filtering options for subscriptions.
// Used by SubscriptionRepository.List to filter results.
type Filter struct {
	SubscriberID int    // Filter by subscriber ID (0 = no filter)
	CbuID        int    // Filter by CBU ID (0 = no filter)
	TopicID      string // Filter by topic ID (empty = no filter)
	IsActive     bool   // Filter by active status
}

// QueueRepository defines the persistence interface for queue items.
// Queue items represent pending or retrying message deliveries.
//
// Implementations must be safe for concurrent use and should use
// database transactions where appropriate.
type QueueRepository interface {
	// Load retrieves a queue item by ID.
	// Returns ErrNoData if not found.
	Load(ctx context.Context, id int64) (model.Queue, error)

	// Save creates a new queue item (if Id=0) or updates an existing one.
	// Returns the saved queue item with populated Id.
	Save(ctx context.Context, m *model.Queue) (*model.Queue, error)

	// Delete permanently removes a queue item from storage.
	Delete(ctx context.Context, m *model.Queue) error

	// FindByMessageID finds a queue item for a specific message and subscription.
	// Returns ErrNoData if not found.
	FindByMessageID(ctx context.Context, subscriptionID, messageID int64) (model.Queue, error)

	// FindBySubscriptionID retrieves all queue items for a subscription.
	// Returns empty slice if none found.
	FindBySubscriptionID(ctx context.Context, subscriptionID int64) ([]model.Queue, error)

	// FindPendingItems finds queue items ready for first-time delivery.
	// Items must have status=PENDING and next_retry_at <= now.
	// Results are ordered by created_at ASC (FIFO).
	FindPendingItems(ctx context.Context, limit int) ([]model.Queue, error)

	// FindRetryableItems finds queue items ready for retry.
	// Items must have status=FAILED and next_retry_at <= now.
	// Results are ordered by created_at ASC (oldest failures first).
	FindRetryableItems(ctx context.Context, limit int) ([]model.Queue, error)

	// FindExpiredItems finds queue items that have expired.
	// Items must have expires_at <= now and status != SENT.
	// Results are ordered by expires_at ASC (oldest first).
	FindExpiredItems(ctx context.Context, limit int) ([]model.Queue, error)

	// UpdateNextRetry updates the retry schedule for a queue item.
	// Used by retry middleware to schedule next delivery attempt.
	UpdateNextRetry(ctx context.Context, id int64, nextRetryAt time.Time, attemptCount int) error
}

// MessageRepository defines the persistence interface for published messages.
// Messages are immutable once created and represent the actual message payloads.
type MessageRepository interface {
	// Load retrieves a message by ID.
	// Returns ErrNoData if not found.
	Load(ctx context.Context, id int64) (model.Message, error)

	// Save creates a new message (if Id=0) or updates an existing one.
	// Returns the saved message with populated Id.
	Save(ctx context.Context, m model.Message) (model.Message, error)

	// Delete permanently removes a message from storage.
	// Should only be used for cleanup, not during normal operation.
	Delete(ctx context.Context, m model.Message) error

	// FindOutdatedMessages finds messages older than the specified number of days.
	// Used for cleanup/archival operations.
	FindOutdatedMessages(ctx context.Context, days int) ([]model.Message, error)
}

// SubscriptionRepository defines the persistence interface for subscription mappings.
// Subscriptions connect subscribers to topics, enabling message delivery.
type SubscriptionRepository interface {
	// Load retrieves a subscription by ID.
	// Returns ErrNoData if not found.
	Load(ctx context.Context, id int64) (model.Subscription, error)

	// Save creates a new subscription (if Id=0) or updates an existing one.
	// Returns the saved subscription with populated Id.
	Save(ctx context.Context, m model.Subscription) (model.Subscription, error)

	// FindActive finds active subscriptions matching the criteria.
	// If subscriberID=0, searches all subscribers.
	// If identifier is empty, searches all identifiers.
	FindActive(ctx context.Context, subscriberID int64, identifier string) ([]model.Subscription, error)

	// List retrieves subscriptions matching the filter criteria.
	// Returns empty slice if none found.
	List(ctx context.Context, filter Filter) ([]model.Subscription, error)

	// FindAllActive retrieves all active subscriptions with full details.
	// Returns SubscriptionFull with joined subscriber and topic information.
	FindAllActive(ctx context.Context) ([]model.SubscriptionFull, error)
}

// DLQRepository defines the persistence interface for the Dead Letter Queue.
// The DLQ stores messages that failed delivery after all retry attempts.
type DLQRepository interface {
	// Load retrieves a DLQ item by ID.
	// Returns ErrNoData if not found.
	Load(ctx context.Context, id int64) (model.DeadLetterQueue, error)

	// Save creates a new DLQ item (if Id=0) or updates an existing one.
	// Returns the saved DLQ item with populated Id.
	Save(ctx context.Context, m model.DeadLetterQueue) (model.DeadLetterQueue, error)

	// Delete permanently removes a DLQ item from storage.
	// Should only be used after successful resolution or manual cleanup.
	Delete(ctx context.Context, m model.DeadLetterQueue) error

	// FindBySubscription retrieves DLQ items for a specific subscription.
	// Results are ordered by created_at DESC (newest first).
	FindBySubscription(ctx context.Context, subscriptionID int64, limit int) ([]model.DeadLetterQueue, error)

	// FindUnresolved retrieves unresolved DLQ items.
	// Results are ordered by created_at ASC (oldest first).
	FindUnresolved(ctx context.Context, limit int) ([]model.DeadLetterQueue, error)

	// FindOlderThan retrieves DLQ items older than the specified threshold.
	// Useful for identifying stuck items requiring attention.
	FindOlderThan(ctx context.Context, threshold time.Duration, limit int) ([]model.DeadLetterQueue, error)

	// FindByMessageID retrieves a DLQ item for a specific message.
	// Returns ErrNoData if not found.
	FindByMessageID(ctx context.Context, messageID int64) (model.DeadLetterQueue, error)

	// GetStats retrieves DLQ statistics including total count, unresolved count,
	// resolution rate, and average age.
	GetStats(ctx context.Context) (model.DLQStats, error)

	// CountUnresolved returns the count of unresolved DLQ items.
	// Useful for dashboard widgets and monitoring.
	CountUnresolved(ctx context.Context) (int, error)
}

// PublisherRepository defines the persistence interface for publisher configurations.
// Publishers represent message sources in the system.
type PublisherRepository interface {
	// Load retrieves a publisher by ID.
	// Returns ErrNoData if not found.
	Load(ctx context.Context, id int64) (model.Publisher, error)

	// Save creates a new publisher (if Id=0) or updates an existing one.
	// Returns the saved publisher with populated Id.
	Save(ctx context.Context, m model.Publisher) (model.Publisher, error)

	// GetByPublisherCode retrieves a publisher by its unique code.
	// Returns ErrNoData if not found.
	GetByPublisherCode(ctx context.Context, publisherCode string) (model.Publisher, error)
}

// SubscriberRepository defines the persistence interface for subscriber configurations.
// Subscribers represent message consumers with webhook URLs for delivery.
type SubscriberRepository interface {
	// Load retrieves a subscriber by ID.
	// Returns ErrNoData if not found.
	Load(ctx context.Context, id int64) (model.Subscriber, error)

	// Save creates a new subscriber (if Id=0) or updates an existing one.
	// Returns the saved subscriber with populated Id.
	Save(ctx context.Context, m model.Subscriber) (model.Subscriber, error)

	// FindByClientID retrieves a subscriber by client ID.
	// Returns ErrNoData if not found.
	FindByClientID(ctx context.Context, clientID int64) (model.Subscriber, error)

	// FindByName retrieves a subscriber by name.
	// Returns ErrNoData if not found.
	FindByName(ctx context.Context, name string) (model.Subscriber, error)
}

// TopicRepository defines the persistence interface for topic configurations.
// Topics represent message categories for pub/sub routing.
type TopicRepository interface {
	// Load retrieves a topic by ID.
	// Returns ErrNoData if not found.
	Load(ctx context.Context, id int64) (model.Topic, error)

	// Save creates a new topic (if Id=0) or updates an existing one.
	// Returns the saved topic with populated Id.
	Save(ctx context.Context, m model.Topic) (model.Topic, error)

	// GetByTopicCode retrieves a topic by its unique code.
	// Returns ErrNoData if not found.
	GetByTopicCode(ctx context.Context, topicCode string) (model.Topic, error)
}
