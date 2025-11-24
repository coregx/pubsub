package relica

import (
	"database/sql"

	"github.com/coregx/pubsub"
)

// Repositories holds all repository implementations.
type Repositories struct {
	Queue        pubsub.QueueRepository
	Message      pubsub.MessageRepository
	Subscription pubsub.SubscriptionRepository
	DLQ          pubsub.DLQRepository
	Publisher    pubsub.PublisherRepository
	Subscriber   pubsub.SubscriberRepository
	Topic        pubsub.TopicRepository
}

// NewRepositories creates all repository implementations using Relica.
//
// The db parameter should be an *sql.DB connected to MySQL, PostgreSQL, or SQLite.
// The driverName should be "mysql", "postgres", or "sqlite3".
// The table prefix defaults to "pubsub_" but can be customized.
func NewRepositories(db *sql.DB, driverName string) *Repositories {
	return &Repositories{
		Queue:        NewQueueRepository(db, driverName),
		Message:      NewMessageRepository(db, driverName),
		Subscription: NewSubscriptionRepository(db, driverName),
		DLQ:          NewDLQRepository(db, driverName),
		Publisher:    NewPublisherRepository(db, driverName),
		Subscriber:   NewSubscriberRepository(db, driverName),
		Topic:        NewTopicRepository(db, driverName),
	}
}

// NewRepositoriesWithPrefix creates all repository implementations with a custom table prefix.
func NewRepositoriesWithPrefix(db *sql.DB, driverName, prefix string) *Repositories {
	return &Repositories{
		Queue:        NewQueueRepositoryWithPrefix(db, driverName, prefix),
		Message:      NewMessageRepositoryWithPrefix(db, driverName, prefix),
		Subscription: NewSubscriptionRepositoryWithPrefix(db, driverName, prefix),
		DLQ:          NewDLQRepositoryWithPrefix(db, driverName, prefix),
		Publisher:    NewPublisherRepositoryWithPrefix(db, driverName, prefix),
		Subscriber:   NewSubscriberRepositoryWithPrefix(db, driverName, prefix),
		Topic:        NewTopicRepositoryWithPrefix(db, driverName, prefix),
	}
}
