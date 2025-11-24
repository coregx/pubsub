// Package pubsub provides a production-ready Pub/Sub library and standalone service for Go
// with reliable message delivery, retry logic, and Dead Letter Queue (DLQ) support.
//
// Works both as a library for embedding in your application AND as a standalone microservice
// with REST API.
//
// # Features
//
//   - Reliable Message Delivery with guaranteed delivery and exponential backoff retry
//   - Exponential Backoff: 30s → 1m → 2m → 4m → 8m → 16m → 30m (max)
//   - Dead Letter Queue (DLQ) automatically handles failed messages after 5 attempts
//   - DLQ Statistics for tracking failure reasons and resolution metrics
//   - Domain-Driven Design with rich domain models containing business logic
//   - Repository Pattern for clean data access abstraction
//   - Options Pattern for modern Go API design (2025 best practices)
//   - Pluggable architecture: bring your own Logger, Notification system
//   - Multi-Database Support: MySQL, PostgreSQL, SQLite via Relica adapters
//   - Zero Dependencies: uses Relica query builder (no ORM bloat)
//   - Embedded Migrations for easy database setup
//   - Docker Ready: production Dockerfile with multi-stage builds
//   - Cloud Native: 12-factor app, ENV config, health checks
//   - Battle-tested in FreiCON Railway Management System
//
// # Quick Start
//
// # Option 1: As Embedded Library
//
// First, apply the database migrations:
//
//	import (
//	    "database/sql"
//	    "github.com/coregx/pubsub"
//	    "github.com/coregx/pubsub/adapters/relica"
//	    "github.com/coregx/pubsub/migrations"
//	    _ "github.com/go-sql-driver/mysql"
//	)
//
//	// Connect to database
//	db, _ := sql.Open("mysql", "user:pass@tcp(localhost:3306)/pubsub?parseTime=true")
//
//	// Apply embedded migrations
//	if err := migrations.ApplyAll(db); err != nil {
//	    log.Fatal(err)
//	}
//
// Use production-ready Relica adapters:
//
//	// Create all repositories at once
//	repos := relica.NewRepositories(db, "mysql")
//
//	// Create services with Options Pattern
//	publisher, _ := pubsub.NewPublisher(
//	    pubsub.WithPublisherRepositories(
//	        repos.Message, repos.Queue, repos.Subscription, repos.Topic,
//	    ),
//	    pubsub.WithPublisherLogger(logger),
//	)
//
//	// Create worker
//	worker, _ := pubsub.NewQueueWorker(
//	    pubsub.WithRepositories(repos.Queue, repos.Message, repos.Subscription, repos.DLQ),
//	    pubsub.WithDelivery(transmitterProvider, gateway),
//	    pubsub.WithLogger(logger),
//	)
//
//	// Run worker (processes queue every 30 seconds)
//	ctx := context.Background()
//	worker.Run(ctx, 30*time.Second)
//
// Publish a message:
//
//	result, err := publisher.Publish(ctx, pubsub.PublishRequest{
//	    TopicCode:  "user.signup",
//	    Identifier: "user-123",
//	    Data:       `{"userId": 123, "email": "user@example.com"}`,
//	})
//
// # Option 2: As Standalone Service
//
// Run the standalone PubSub server with Docker:
//
//	cd cmd/pubsub-server
//	docker-compose up -d
//
// Access REST API at http://localhost:8080:
//
//	# Publish message
//	curl -X POST http://localhost:8080/api/v1/publish \
//	  -H "Content-Type: application/json" \
//	  -d '{"topicCode":"user.signup","identifier":"user-123","data":{"userId":123}}'
//
//	# Health check
//	curl http://localhost:8080/api/v1/health
//
// See cmd/pubsub-server/README.md for full API documentation
//
// # Architecture
//
// The library follows Clean Architecture and Domain-Driven Design principles:
//
//	┌─────────────────────────────────────┐
//	│         Application Layer           │
//	│  (Publisher, SubscriptionManager,   │
//	│   QueueWorker, REST API)            │
//	└─────────────┬───────────────────────┘
//	              │
//	┌─────────────▼───────────────────────┐
//	│         Domain Layer                │
//	│  (Rich models with business logic)  │
//	└─────────────┬───────────────────────┘
//	              │
//	┌─────────────▼───────────────────────┐
//	│       Relica Adapters               │
//	│  (Production-ready implementations) │
//	└─────────────┬───────────────────────┘
//	              │
//	┌─────────────▼───────────────────────┐
//	│    Database (MySQL/PostgreSQL/      │
//	│             SQLite)                 │
//	└─────────────────────────────────────┘
//
// Key principles:
//   - Domain models contain business logic (Queue.MarkFailed, Queue.ShouldRetry, etc.)
//   - Repository Pattern abstracts database operations
//   - Dependency Inversion via interfaces (Logger, Notification, Repositories)
//   - Options Pattern for service configuration (2025 best practices)
//   - Relica adapters provide zero-dependency database access
//
// # Message Flow
//
//  1. PUBLISH
//     Publisher → Topic → Create Message
//     → Find Active Subscriptions
//     → Create Queue Items (one per subscription)
//
//  2. WORKER (Background)
//     QueueWorker → Find Pending/Retryable Items (batch)
//     → Deliver to Subscribers (via webhooks/gateway)
//     → On Success: Mark as SENT
//     → On Failure: Retry with exponential backoff
//     → After 5 failures: Move to DLQ
//
//  3. DLQ (Dead Letter Queue)
//     Failed items → Manual review
//     → Resolve or Delete
//
// # Retry Strategy
//
// Failed message deliveries are automatically retried with exponential backoff:
//
//	Attempt 1: Immediate
//	Attempt 2: +30 seconds
//	Attempt 3: +1 minute
//	Attempt 4: +2 minutes
//	Attempt 5: +4 minutes
//	Attempt 6: +8 minutes (moves to DLQ after this)
//
// After 5 failed attempts, messages are automatically moved to the Dead Letter Queue (DLQ)
// for manual inspection and resolution.
//
// # Database Schema
//
// The library requires 7 database tables (created via embedded migrations):
//
//	pubsub_topic          - Topics for pub/sub messaging
//	pubsub_publisher      - Publisher configurations
//	pubsub_subscriber     - Subscriber configurations with webhook URLs
//	pubsub_subscription   - Subscription mappings (subscriber + topic)
//	pubsub_message        - Published messages
//	pubsub_queue          - Delivery queue with retry state
//	pubsub_dlq            - Dead Letter Queue for failed messages
//
// Supports MySQL, PostgreSQL, and SQLite via Relica adapters.
// Table prefix can be customized (default: "pubsub_").
//
// # Examples
//
// See the examples/ directory for complete working examples including:
//
//   - Basic usage with database/sql
//   - Custom logger integration
//   - Repository implementations
//
// For detailed documentation, see README.md and pkg.go.dev.
package pubsub
