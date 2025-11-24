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

// TopicRepository implements pubsub.TopicRepository using Relica ORM.
type TopicRepository struct {
	db          *relica.DB
	tablePrefix string
}

// NewTopicRepository creates a new TopicRepository with default table prefix.
func NewTopicRepository(sqlDB *sql.DB, driverName string) *TopicRepository {
	return &TopicRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: "pubsub_"}
}

// NewTopicRepositoryWithPrefix creates a new TopicRepository with custom table prefix.
func NewTopicRepositoryWithPrefix(sqlDB *sql.DB, driverName, prefix string) *TopicRepository {
	return &TopicRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: prefix}
}

func (r *TopicRepository) tableName() string {
	return r.tablePrefix + "topic"
}

// Load retrieves a topic by ID.
func (r *TopicRepository) Load(ctx context.Context, id int64) (model.Topic, error) {
	var topic model.Topic
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("id = ?", id).One(&topic)
	if errors.Is(err, sql.ErrNoRows) {
		return topic, pubsub.ErrNoData
	}
	if err != nil {
		return topic, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to load topic", err)
	}
	return topic, nil
}

// Save creates or updates a topic.
func (r *TopicRepository) Save(ctx context.Context, m model.Topic) (model.Topic, error) {
	if m.ID == 0 {
		// Insert using Model() API
		err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Insert()
		if err != nil {
			return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to insert topic", err)
		}
		return m, nil
	}

	// Update using Model() API
	err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Update()
	if err != nil {
		return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to update topic", err)
	}
	return m, nil
}

// GetByTopicCode retrieves a topic by its unique code.
func (r *TopicRepository) GetByTopicCode(ctx context.Context, topicCode string) (model.Topic, error) {
	var topic model.Topic
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("topic_code = ?", topicCode).One(&topic)
	if errors.Is(err, sql.ErrNoRows) {
		return topic, pubsub.ErrNoData
	}
	if err != nil {
		return topic, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find topic by code", err)
	}
	return topic, nil
}
