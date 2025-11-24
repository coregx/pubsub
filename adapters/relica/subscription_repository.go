package relica

import (
	"context"
	"database/sql"
	"errors"

	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/model"
	"github.com/coregx/relica"
)

// SubscriptionRepository implements pubsub.SubscriptionRepository using Relica ORM.
type SubscriptionRepository struct {
	db          *relica.DB
	tablePrefix string
}

// NewSubscriptionRepository creates a new SubscriptionRepository with default table prefix.
func NewSubscriptionRepository(sqlDB *sql.DB, driverName string) *SubscriptionRepository {
	return &SubscriptionRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: "pubsub_"}
}

// NewSubscriptionRepositoryWithPrefix creates a new SubscriptionRepository with custom table prefix.
func NewSubscriptionRepositoryWithPrefix(sqlDB *sql.DB, driverName, prefix string) *SubscriptionRepository {
	return &SubscriptionRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: prefix}
}

func (r *SubscriptionRepository) tableName() string {
	return r.tablePrefix + "subscription"
}

// Load retrieves a subscription by ID.
func (r *SubscriptionRepository) Load(ctx context.Context, id int64) (model.Subscription, error) {
	var sub model.Subscription
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("id = ?", id).One(&sub)
	if errors.Is(err, sql.ErrNoRows) {
		return sub, pubsub.ErrNoData
	}
	if err != nil {
		return sub, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to load subscription", err)
	}
	return sub, nil
}

// Save creates or updates a subscription.
func (r *SubscriptionRepository) Save(ctx context.Context, m model.Subscription) (model.Subscription, error) {
	if m.ID == 0 {
		// Insert using Model() API
		err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Insert()
		if err != nil {
			return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to insert subscription", err)
		}
		return m, nil
	}
	// Update using Model() API
	err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Update()
	if err != nil {
		return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to update subscription", err)
	}
	return m, nil
}

// FindActive finds active subscriptions matching the criteria.
func (r *SubscriptionRepository) FindActive(ctx context.Context, subscriberID int64, identifier string) ([]model.Subscription, error) {
	var subs []model.Subscription
	q := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("is_active = ?", true)
	if subscriberID > 0 {
		q = q.Where("subscriber_id = ?", subscriberID)
	}
	if identifier != "" {
		q = q.Where("identifier = ?", identifier)
	}
	err := q.WithContext(ctx).All(&subs)
	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find active subscriptions", err)
	}
	if len(subs) == 0 {
		return nil, pubsub.ErrNoData
	}
	return subs, nil
}

// List retrieves subscriptions matching the filter criteria.
func (r *SubscriptionRepository) List(ctx context.Context, filter pubsub.Filter) ([]model.Subscription, error) {
	var subs []model.Subscription
	q := r.db.WithContext(ctx).Select("*").From(r.tableName())
	if filter.SubscriberID > 0 {
		q = q.Where("subscriber_id = ?", filter.SubscriberID)
	}
	if filter.TopicID != "" {
		q = q.Where("topic_id = ?", filter.TopicID)
	}
	if filter.IsActive {
		q = q.Where("is_active = ?", true)
	}
	err := q.WithContext(ctx).All(&subs)
	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to list subscriptions", err)
	}
	if len(subs) == 0 {
		return nil, pubsub.ErrNoData
	}
	return subs, nil
}

// FindAllActive retrieves all active subscriptions with full details.
func (r *SubscriptionRepository) FindAllActive(ctx context.Context) ([]model.SubscriptionFull, error) {
	var subs []model.SubscriptionFull
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("is_active = ?", true).All(&subs)
	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find all active subscriptions", err)
	}
	if len(subs) == 0 {
		return nil, pubsub.ErrNoData
	}
	return subs, nil
}
