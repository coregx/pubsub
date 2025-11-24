package pubsub

import (
	"fmt"

	"github.com/coregx/pubsub/retry"
)

// Option is a function that configures a QueueWorker.
// Used with the Options Pattern (2025 Go best practice) for flexible service construction.
//
// Example:
//
//	worker, err := pubsub.NewQueueWorker(
//	    pubsub.WithRepositories(queueRepo, msgRepo, subRepo, dlqRepo),
//	    pubsub.WithDelivery(transmitterProvider, gateway),
//	    pubsub.WithLogger(logger),
//	    pubsub.WithBatchSize(200), // optional
//	)
type Option func(*QueueWorker) error

// WithRepositories sets the required repository dependencies for the queue worker.
// All four repositories are required and must not be nil.
//
// This is a required option for NewQueueWorker.
//
// Parameters:
//   - queueRepo: Queue item persistence
//   - messageRepo: Message persistence
//   - subscriptionRepo: Subscription persistence
//   - dlqRepo: Dead Letter Queue persistence
func WithRepositories(
	queueRepo QueueRepository,
	messageRepo MessageRepository,
	subscriptionRepo SubscriptionRepository,
	dlqRepo DLQRepository,
) Option {
	return func(w *QueueWorker) error {
		if queueRepo == nil {
			return fmt.Errorf("queueRepo cannot be nil")
		}
		if messageRepo == nil {
			return fmt.Errorf("messageRepo cannot be nil")
		}
		if subscriptionRepo == nil {
			return fmt.Errorf("subscriptionRepo cannot be nil")
		}
		if dlqRepo == nil {
			return fmt.Errorf("dlqRepo cannot be nil")
		}

		w.qr = queueRepo
		w.mr = messageRepo
		w.sr = subscriptionRepo
		w.dlqr = dlqRepo
		return nil
	}
}

// WithDelivery sets the message delivery dependencies for the queue worker.
// Both provider and gateway are required and must not be nil.
//
// This is a required option for NewQueueWorker.
//
// Parameters:
//   - transmitterProvider: Resolves subscriber webhook URLs
//   - gateway: Handles actual HTTP/gRPC message delivery
func WithDelivery(
	transmitterProvider TransmitterProvider,
	gateway MessageDeliveryGateway,
) Option {
	return func(w *QueueWorker) error {
		if transmitterProvider == nil {
			return fmt.Errorf("transmitterProvider cannot be nil")
		}
		if gateway == nil {
			return fmt.Errorf("gateway cannot be nil")
		}

		w.transmitterProvider = transmitterProvider
		w.gateway = gateway
		return nil
	}
}

// WithLogger sets the logger instance for the queue worker.
// Logger is required and must not be nil.
//
// This is a required option for NewQueueWorker.
//
// Use NoopLogger for silent operation or implement Logger interface
// to integrate with your logging system (zap, logrus, etc.).
func WithLogger(logger Logger) Option {
	return func(w *QueueWorker) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		w.logger = logger
		return nil
	}
}

// WithRetryStrategy sets a custom retry strategy for the queue worker.
// This is an optional configuration - if not provided, retry.DefaultStrategy() will be used.
//
// The default strategy implements exponential backoff: 30s → 1m → 2m → 4m → 8m → 16m → 30m (max).
//
// Use this option to customize:
//   - Retry delays (backoff schedule)
//   - Maximum retry attempts before DLQ
//   - DLQ threshold
func WithRetryStrategy(strategy retry.Strategy) Option {
	return func(w *QueueWorker) error {
		w.retryStrategy = strategy
		return nil
	}
}

// WithBatchSize sets the number of queue items to process per batch.
// This is an optional configuration - default is 100 items per batch.
//
// Must be > 0. Larger batches improve throughput but use more memory.
// Smaller batches reduce latency and memory usage.
//
// Recommended values:
//   - Low volume: 50-100
//   - Medium volume: 100-500
//   - High volume: 500-1000
func WithBatchSize(size int) Option {
	return func(w *QueueWorker) error {
		if size <= 0 {
			return fmt.Errorf("batch size must be > 0, got %d", size)
		}
		w.batchSize = size
		return nil
	}
}

// WithNotifications sets an optional notification service for the queue worker.
// This is an optional configuration - if not provided, NoOpNotificationService will be used (no notifications).
//
// The notification service receives callbacks for:
//   - Delivery failures (every failed attempt)
//   - DLQ item additions (when message exhausts retries)
//
// Use this to integrate with alerting systems (email, Slack, PagerDuty, etc.).
func WithNotifications(service NotificationService) Option {
	return func(w *QueueWorker) error {
		if service == nil {
			return fmt.Errorf("notification service cannot be nil")
		}
		w.notificationService = service
		return nil
	}
}
