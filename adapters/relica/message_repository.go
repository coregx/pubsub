package relica

import (
	"context"
	"database/sql"
	"errors"

	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/model"
	"github.com/coregx/relica"
)

// MessageRepository implements pubsub.MessageRepository using Relica.
type MessageRepository struct {
	db          *relica.DB
	tablePrefix string
}

// NewMessageRepository creates a new MessageRepository with default table prefix.
func NewMessageRepository(sqlDB *sql.DB, driverName string) *MessageRepository {
	return &MessageRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: "pubsub_"}
}

// NewMessageRepositoryWithPrefix creates a new MessageRepository with custom table prefix.
func NewMessageRepositoryWithPrefix(sqlDB *sql.DB, driverName, prefix string) *MessageRepository {
	return &MessageRepository{db: relica.WrapDB(sqlDB, driverName), tablePrefix: prefix}
}

func (r *MessageRepository) tableName() string {
	return r.tablePrefix + "message"
}

// Load retrieves a message by ID.
func (r *MessageRepository) Load(ctx context.Context, id int64) (model.Message, error) {
	var msg model.Message
	err := r.db.WithContext(ctx).Select("*").From(r.tableName()).Where("id = ?", id).One(&msg)
	if errors.Is(err, sql.ErrNoRows) {
		return msg, pubsub.ErrNoData
	}
	if err != nil {
		return msg, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to load message", err)
	}
	return msg, nil
}

// Save creates or updates a message.
func (r *MessageRepository) Save(ctx context.Context, m model.Message) (model.Message, error) {
	if m.ID == 0 {
		// Insert new message using Model() API
		err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Insert()
		if err != nil {
			return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to insert message", err)
		}
		// m.ID is auto-populated by Model().Insert()
		return m, nil
	}

	// Update existing message
	err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Update()
	if err != nil {
		return m, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to update message", err)
	}
	return m, nil
}

// Delete removes a message.
func (r *MessageRepository) Delete(ctx context.Context, m model.Message) error {
	// Delete using Model() API - auto WHERE id = ?
	err := r.db.WithContext(ctx).Model(&m).Table(r.tableName()).Delete()
	if err != nil {
		return pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to delete message", err)
	}
	return nil
}

// FindOutdatedMessages finds messages older than the specified number of days.
func (r *MessageRepository) FindOutdatedMessages(ctx context.Context, days int) ([]model.Message, error) {
	var messages []model.Message
	err := r.db.WithContext(ctx).Select("*").
		From(r.tableName()).
		Where("created_at < DATE_SUB(NOW(), INTERVAL ? DAY)", days).
		OrderBy("created_at ASC").
		WithContext(ctx).
		All(&messages)
	if err != nil {
		return nil, pubsub.NewErrorWithCause(pubsub.ErrCodeDatabase, "failed to find outdated messages", err)
	}
	if len(messages) == 0 {
		return nil, pubsub.ErrNoData
	}
	return messages, nil
}
