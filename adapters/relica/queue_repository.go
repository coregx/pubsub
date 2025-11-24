package relica

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/model"
	"github.com/coregx/relica"
)

// QueueRepository implements pubsub.QueueRepository using Relica.
type QueueRepository struct {
	db          *relica.DB
	tablePrefix string
}

// NewQueueRepository creates a new QueueRepository with default table prefix.
func NewQueueRepository(sqlDB *sql.DB, driverName string) *QueueRepository {
	return &QueueRepository{
		db:          relica.WrapDB(sqlDB, driverName),
		tablePrefix: "pubsub_",
	}
}

// NewQueueRepositoryWithPrefix creates a new QueueRepository with custom table prefix.
func NewQueueRepositoryWithPrefix(sqlDB *sql.DB, driverName, prefix string) *QueueRepository {
	return &QueueRepository{
		db:          relica.WrapDB(sqlDB, driverName),
		tablePrefix: prefix,
	}
}

func (r *QueueRepository) tableName() string {
	return r.tablePrefix + "queue"
}

// Load retrieves a queue item by ID.
func (r *QueueRepository) Load(ctx context.Context, id int64) (model.Queue, error) {
	var queue model.Queue

	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where("id = ?", id).
		WithContext(ctx).
		One(&queue)

	if errors.Is(err, sql.ErrNoRows) {
		return queue, pubsub.ErrNoData
	}
	if err != nil {
		return queue, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to load queue", err)
	}

	return queue, nil
}

// Save creates or updates a queue item.
func (r *QueueRepository) Save(ctx context.Context, m *model.Queue) (*model.Queue, error) {
	if m.ID == 0 {
		// Insert using Model() API - auto-populates m.ID
		err := r.db.WithContext(ctx).Model(m).Table(r.tableName()).Insert()
		if err != nil {
			return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to insert queue", err)
		}
		return m, nil
	}

	// Update using Model() API - auto WHERE id = ?
	err := r.db.WithContext(ctx).Model(m).Table(r.tableName()).Update()
	if err != nil {
		return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to update queue", err)
	}

	return m, nil
}

// Delete removes a queue item.
func (r *QueueRepository) Delete(ctx context.Context, m *model.Queue) error {
	// Delete using Model() API - auto WHERE id = ?
	err := r.db.WithContext(ctx).Model(m).Table(r.tableName()).Delete()
	if err != nil {
		return pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to delete queue", err)
	}

	return nil
}

// FindByMessageID retrieves a queue item by message and subscription IDs.
func (r *QueueRepository) FindByMessageID(ctx context.Context, subscriptionID, messageID int64) (model.Queue, error) {
	var queue model.Queue

	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where("subscription_id = ? AND message_id = ?", subscriptionID, messageID).
		WithContext(ctx).
		One(&queue)

	if errors.Is(err, sql.ErrNoRows) {
		return queue, pubsub.ErrNoData
	}
	if err != nil {
		return queue, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find queue by message", err)
	}

	return queue, nil
}

// FindBySubscriptionID retrieves all queue items for a subscription.
func (r *QueueRepository) FindBySubscriptionID(ctx context.Context, subscriptionID int64) ([]model.Queue, error) {
	var queues []model.Queue

	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where("subscription_id = ?", subscriptionID).
		OrderBy("created_at DESC").
		WithContext(ctx).
		All(&queues)

	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find queues by subscription", err)
	}

	if len(queues) == 0 {
		return nil, pubsub.ErrNoData
	}

	return queues, nil
}

// FindPendingItems retrieves pending queue items ready for first delivery.
func (r *QueueRepository) FindPendingItems(ctx context.Context, limit int) ([]model.Queue, error) {
	var queues []model.Queue

	now := time.Now()

	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where("status = ? AND next_retry_at <= ?", model.QueueStatusPending, now).
		OrderBy("created_at ASC").
		Limit(int64(limit)).
		WithContext(ctx).
		All(&queues)

	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find pending items", err)
	}

	if len(queues) == 0 {
		return nil, pubsub.ErrNoData
	}

	return queues, nil
}

// FindRetryableItems retrieves failed queue items ready for retry.
func (r *QueueRepository) FindRetryableItems(ctx context.Context, limit int) ([]model.Queue, error) {
	var queues []model.Queue

	now := time.Now()

	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where("status = ? AND next_retry_at <= ?", model.QueueStatusFailed, now).
		OrderBy("created_at ASC").
		Limit(int64(limit)).
		WithContext(ctx).
		All(&queues)

	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find retryable items", err)
	}

	if len(queues) == 0 {
		return nil, pubsub.ErrNoData
	}

	return queues, nil
}

// FindExpiredItems retrieves expired queue items that should be cleaned up.
func (r *QueueRepository) FindExpiredItems(ctx context.Context, limit int) ([]model.Queue, error) {
	var queues []model.Queue

	now := time.Now()

	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where("expires_at <= ? AND status != ?", now, model.QueueStatusSent).
		OrderBy("expires_at ASC").
		Limit(int64(limit)).
		WithContext(ctx).
		All(&queues)

	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find expired items", err)
	}

	if len(queues) == 0 {
		return nil, pubsub.ErrNoData
	}

	return queues, nil
}

// UpdateNextRetry updates the next retry time and attempt count.
func (r *QueueRepository) UpdateNextRetry(ctx context.Context, id int64, nextRetryAt time.Time, attemptCount int) error {
	_, err := r.db.WithContext(ctx).Update(r.tableName()).
		Set(map[string]interface{}{
			"next_retry_at": nextRetryAt,
			"attempt_count": attemptCount,
		}).
		Where("id = ?", id).
		WithContext(ctx).
		Execute()

	if err != nil {
		return pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to update next retry", err)
	}

	return nil
}
