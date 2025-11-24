package relica

import (
	"context"
	"database/sql"
	"errors"

	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/model"
	"github.com/coregx/relica"
)

// SubscriberRepository implements pubsub.SubscriberRepository using Relica ORM.
type SubscriberRepository struct {
	db          *relica.DB
	tablePrefix string
}

// NewSubscriberRepository creates a new SubscriberRepository with default table prefix.
func NewSubscriberRepository(sqlDB *sql.DB, driverName string) *SubscriberRepository {
	return &SubscriberRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: "pubsub_"}
}

// NewSubscriberRepositoryWithPrefix creates a new SubscriberRepository with custom table prefix.
func NewSubscriberRepositoryWithPrefix(sqlDB *sql.DB, driverName, prefix string) *SubscriberRepository {
	return &SubscriberRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: prefix}
}

func (r *SubscriberRepository) tableName() string {
	return r.tablePrefix + "subscriber"
}

// Load retrieves a subscriber by ID.
func (r *SubscriberRepository) Load(ctx context.Context, id int64) (model.Subscriber, error) {
	var sub model.Subscriber
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("id = ?", id).One(&sub)
	if errors.Is(err, sql.ErrNoRows) {
		return sub, pubsub.ErrNoData
	}
	if err != nil {
		return sub, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to load subscriber", err)
	}
	return sub, nil
}

// Save creates or updates a subscriber.
func (r *SubscriberRepository) Save(ctx context.Context, m model.Subscriber) (model.Subscriber, error) {
	if m.ID == 0 {
		// Insert using Model() API
		err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Insert()
		if err != nil {
			return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to insert subscriber", err)
		}
		return m, nil
	}

	// Update using Model() API
	err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Update()
	if err != nil {
		return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to update subscriber", err)
	}
	return m, nil
}

// FindByClientID retrieves a subscriber by client ID.
func (r *SubscriberRepository) FindByClientID(ctx context.Context, clientID int64) (model.Subscriber, error) {
	var sub model.Subscriber
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("client_id = ?", clientID).One(&sub)
	if errors.Is(err, sql.ErrNoRows) {
		return sub, pubsub.ErrNoData
	}
	if err != nil {
		return sub, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find subscriber by client_id", err)
	}
	return sub, nil
}

// FindByName retrieves a subscriber by name.
func (r *SubscriberRepository) FindByName(ctx context.Context, name string) (model.Subscriber, error) {
	var sub model.Subscriber
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("name = ?", name).One(&sub)
	if errors.Is(err, sql.ErrNoRows) {
		return sub, pubsub.ErrNoData
	}
	if err != nil {
		return sub, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find subscriber by name", err)
	}
	return sub, nil
}
