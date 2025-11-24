// Package relica provides Relica ORM implementations for PubSub repositories.
//
//nolint:dupl // Repository pattern requires similar implementations for different types
package relica

import (
	"context"
	"database/sql"
	"errors"

	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/model"
	"github.com/coregx/relica"
)

// PublisherRepository implements pubsub.PublisherRepository using Relica ORM.
type PublisherRepository struct {
	db          *relica.DB
	tablePrefix string
}

// NewPublisherRepository creates a new PublisherRepository with default table prefix.
func NewPublisherRepository(sqlDB *sql.DB, driverName string) *PublisherRepository {
	return &PublisherRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: "pubsub_"}
}

// NewPublisherRepositoryWithPrefix creates a new PublisherRepository with custom table prefix.
func NewPublisherRepositoryWithPrefix(sqlDB *sql.DB, driverName, prefix string) *PublisherRepository {
	return &PublisherRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: prefix}
}

func (r *PublisherRepository) tableName() string {
	return r.tablePrefix + "publisher"
}

// Load retrieves a publisher by ID.
func (r *PublisherRepository) Load(ctx context.Context, id int64) (model.Publisher, error) {
	var pub model.Publisher
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("id = ?", id).One(&pub)
	if errors.Is(err, sql.ErrNoRows) {
		return pub, pubsub.ErrNoData
	}
	if err != nil {
		return pub, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to load publisher", err)
	}
	return pub, nil
}

// Save creates or updates a publisher.
func (r *PublisherRepository) Save(ctx context.Context, m model.Publisher) (model.Publisher, error) {
	if m.ID == 0 {
		// Insert using Model() API
		err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Insert()
		if err != nil {
			return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to insert publisher", err)
		}
		return m, nil
	}

	// Update using Model() API
	err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Update()
	if err != nil {
		return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to update publisher", err)
	}
	return m, nil
}

// GetByPublisherCode retrieves a publisher by its unique code.
func (r *PublisherRepository) GetByPublisherCode(ctx context.Context, publisherCode string) (model.Publisher, error) {
	var pub model.Publisher
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("publisher_code = ?", publisherCode).One(&pub)
	if errors.Is(err, sql.ErrNoRows) {
		return pub, pubsub.ErrNoData
	}
	if err != nil {
		return pub, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find publisher by code", err)
	}
	return pub, nil
}
