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

// DLQRepository implements pubsub.DLQRepository using Relica ORM.
type DLQRepository struct {
	db          *relica.DB
	tablePrefix string
}

// NewDLQRepository creates a new DLQRepository with default table prefix.
func NewDLQRepository(sqlDB *sql.DB, driverName string) *DLQRepository {
	return &DLQRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: defaultTablePrefix}
}

// NewDLQRepositoryWithPrefix creates a new DLQRepository with custom table prefix.
func NewDLQRepositoryWithPrefix(sqlDB *sql.DB, driverName, prefix string) *DLQRepository {
	return &DLQRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: prefix}
}

func (r *DLQRepository) tableName() string {
	return r.tablePrefix + "dead_letter_queue"
}

// Load retrieves a DLQ item by ID.
func (r *DLQRepository) Load(ctx context.Context, id int64) (model.DeadLetterQueue, error) {
	var dlq model.DeadLetterQueue
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where(relica.Eq("id", id)).One(&dlq)
	if errors.Is(err, relica.ErrNotFound) {
		return dlq, pubsub.ErrNoData
	}
	if err != nil {
		return dlq, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to load DLQ", err)
	}
	return dlq, nil
}

// Save creates or updates a DLQ item.
func (r *DLQRepository) Save(ctx context.Context, m model.DeadLetterQueue) (model.DeadLetterQueue, error) {
	if m.ID == 0 {
		err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Insert()
		if err != nil {
			return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to insert DLQ", err)
		}
		return m, nil
	}

	err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Update()
	if err != nil {
		return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to update DLQ", err)
	}
	return m, nil
}

// Delete removes a DLQ item.
func (r *DLQRepository) Delete(ctx context.Context, m model.DeadLetterQueue) error {
	err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Delete()
	if err != nil {
		return pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to delete DLQ", err)
	}
	return nil
}

// FindBySubscription retrieves DLQ items for a specific subscription.
func (r *DLQRepository) FindBySubscription(ctx context.Context, subscriptionID int64, limit int) ([]model.DeadLetterQueue, error) {
	var dlqs []model.DeadLetterQueue
	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where(relica.Eq("subscription_id", subscriptionID)).
		OrderBy("created_at DESC").
		Limit(int64(limit)).
		All(&dlqs)
	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find DLQ by subscription", err)
	}
	if len(dlqs) == 0 {
		return nil, pubsub.ErrNoData
	}
	return dlqs, nil
}

// FindUnresolved retrieves unresolved DLQ items.
func (r *DLQRepository) FindUnresolved(ctx context.Context, limit int) ([]model.DeadLetterQueue, error) {
	var dlqs []model.DeadLetterQueue
	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where(relica.Eq("is_resolved", false)).
		OrderBy("created_at ASC").
		Limit(int64(limit)).
		All(&dlqs)
	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find unresolved DLQ items", err)
	}
	if len(dlqs) == 0 {
		return nil, pubsub.ErrNoData
	}
	return dlqs, nil
}

// FindOlderThan retrieves DLQ items older than the specified threshold.
func (r *DLQRepository) FindOlderThan(ctx context.Context, threshold time.Duration, limit int) ([]model.DeadLetterQueue, error) {
	var dlqs []model.DeadLetterQueue
	cutoffTime := time.Now().Add(-threshold)
	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where(relica.LessThan("created_at", cutoffTime)).
		OrderBy("created_at ASC").
		Limit(int64(limit)).
		All(&dlqs)
	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find old DLQ items", err)
	}
	if len(dlqs) == 0 {
		return nil, pubsub.ErrNoData
	}
	return dlqs, nil
}

// FindByMessageID retrieves a DLQ item for a specific message.
func (r *DLQRepository) FindByMessageID(ctx context.Context, messageID int64) (model.DeadLetterQueue, error) {
	var dlq model.DeadLetterQueue
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where(relica.Eq("message_id", messageID)).One(&dlq)
	if errors.Is(err, relica.ErrNotFound) {
		return dlq, pubsub.ErrNoData
	}
	if err != nil {
		return dlq, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find DLQ by message", err)
	}
	return dlq, nil
}

// GetStats retrieves DLQ statistics using Relica Count() for efficiency.
func (r *DLQRepository) GetStats(ctx context.Context) (model.DLQStats, error) {
	var stats model.DLQStats

	totalCount, err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Count()
	if err != nil {
		return stats, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to count total DLQ items", err)
	}
	stats.TotalItems = int(totalCount)

	unresolvedCount, err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where(relica.Eq("is_resolved", false)).Count()
	if err != nil {
		return stats, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to count unresolved DLQ items", err)
	}
	stats.UnresolvedItems = int(unresolvedCount)
	stats.ResolvedItems = stats.TotalItems - stats.UnresolvedItems
	return stats, nil
}

// CountUnresolved returns the count of unresolved DLQ items.
func (r *DLQRepository) CountUnresolved(ctx context.Context) (int, error) {
	count, err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where(relica.Eq("is_resolved", false)).Count()
	if err != nil {
		return 0, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to count unresolved DLQ items", err)
	}
	return int(count), nil
}
