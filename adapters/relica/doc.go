// Package relica provides repository implementations using Relica query builder.
//
// Relica (github.com/coregx/relica) is a lightweight, type-safe database query builder
// for Go with zero production dependencies.
//
// This package provides production-ready implementations of all pubsub repository interfaces:
//   - QueueRepository
//   - MessageRepository
//   - SubscriptionRepository
//   - DLQRepository
//   - PublisherRepository
//   - SubscriberRepository
//   - TopicRepository
//
// Example usage:
//
//	import (
//	    "database/sql"
//	    "github.com/coregx/pubsub"
//	    "github.com/coregx/pubsub/adapters/relica"
//	    _ "github.com/go-sql-driver/mysql"
//	)
//
//	// Open database connection
//	db, err := sql.Open("mysql", "user:pass@tcp(localhost:3306)/pubsub_db?parseTime=true")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create repositories (driverName should be "mysql", "postgres", or "sqlite3")
//	repos := relica.NewRepositories(db, "mysql")
//
//	// Create services
//	worker, err := pubsub.NewQueueWorker(
//	    pubsub.WithRepositories(repos.Queue, repos.Message, repos.Subscription, repos.DLQ),
//	    pubsub.WithDelivery(transmitterProvider, gateway),
//	    pubsub.WithLogger(logger),
//	)
package relica
