package pubsub

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/coregx/pubsub/model"
	"github.com/coregx/pubsub/retry"
)

// MessageDeliveryGateway defines the interface for delivering messages to subscriber webhooks.
// This interface avoids circular dependency with the transmitter package while enabling
// flexible delivery implementations (HTTP webhooks, gRPC, message queues, etc.).
//
// Implementations should handle HTTP transport, retries at the transport level,
// and return errors for failed deliveries to trigger the retry mechanism.
type MessageDeliveryGateway interface {
	// DeliverMessage sends a message to the subscriber's webhook endpoint.
	// Returns error if delivery fails (network error, non-2xx response, timeout).
	DeliverMessage(ctx context.Context, callbackURL string, message *model.DataMessage) error
}

// TransmitterProvider provides subscriber callback URL resolution without circular dependency.
// This interface decouples the worker from the transmitter/subscriber details.
//
// Implementations typically fetch webhook URLs from subscriber configuration.
type TransmitterProvider interface {
	// GetCallbackUrl retrieves the webhook URL for a subscriber.
	// Returns ErrNoData if subscriber not found.
	GetCallbackUrl(ctx context.Context, subscriberID int64) (string, error)
}

// QueueWorker processes the message delivery queue with automatic retry logic.
// It handles pending messages, failed retries, and Dead Letter Queue management.
//
// The worker runs continuously in the background, processing batches at regular intervals.
// It implements exponential backoff retry strategy and moves permanently failed messages
// to the Dead Letter Queue (DLQ) for manual inspection.
//
// Key responsibilities:
//   - Process pending queue items (first delivery attempt)
//   - Retry failed deliveries with exponential backoff
//   - Move exhausted retries to DLQ
//   - Clean up expired queue items
//   - Send notifications for delivery failures and DLQ additions
//
// Thread safety: Safe for concurrent use. Each batch is processed sequentially.
type QueueWorker struct {
	qr                  QueueRepository
	mr                  MessageRepository
	sr                  SubscriptionRepository
	dlqr                DLQRepository
	transmitterProvider TransmitterProvider
	gateway             MessageDeliveryGateway
	retryStrategy       retry.Strategy
	logger              Logger
	notificationService NotificationService
	batchSize           int
}

// NewQueueWorker creates a new queue worker with the provided options.
//
// Required options:
//   - WithRepositories: queue, message, subscription, and DLQ repositories
//   - WithDelivery: transmitter provider and message delivery gateway
//   - WithLogger: logger instance
//
// Optional options:
//   - WithRetryStrategy: custom retry strategy (default: retry.DefaultStrategy())
//   - WithBatchSize: batch processing size (default: 100)
//
// Example:
//
//	worker, err := pubsub.NewQueueWorker(
//	    pubsub.WithRepositories(queueRepo, msgRepo, subRepo, dlqRepo),
//	    pubsub.WithDelivery(transmitterProvider, gateway),
//	    pubsub.WithLogger(logger),
//	    pubsub.WithBatchSize(200), // optional
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewQueueWorker(opts ...Option) (*QueueWorker, error) {
	// Default configuration
	w := &QueueWorker{
		retryStrategy:       retry.DefaultStrategy(),
		batchSize:           100,
		notificationService: &NoOpNotificationService{}, // Default: no notifications
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(w); err != nil {
			return nil, NewErrorWithCause(ErrCodeConfiguration, "failed to apply option", err)
		}
	}

	// Validate required dependencies
	if w.qr == nil {
		return nil, NewError(ErrCodeConfiguration, "QueueRepository is required (use WithRepositories)")
	}
	if w.mr == nil {
		return nil, NewError(ErrCodeConfiguration, "MessageRepository is required (use WithRepositories)")
	}
	if w.sr == nil {
		return nil, NewError(ErrCodeConfiguration, "SubscriptionRepository is required (use WithRepositories)")
	}
	if w.dlqr == nil {
		return nil, NewError(ErrCodeConfiguration, "DLQRepository is required (use WithRepositories)")
	}
	if w.transmitterProvider == nil {
		return nil, NewError(ErrCodeConfiguration, "TransmitterProvider is required (use WithDelivery)")
	}
	if w.gateway == nil {
		return nil, NewError(ErrCodeConfiguration, "MessageDeliveryGateway is required (use WithDelivery)")
	}
	if w.logger == nil {
		return nil, NewError(ErrCodeConfiguration, "Logger is required (use WithLogger)")
	}

	return w, nil
}

// ProcessPendingItems processes pending queue items ready for first delivery attempt.
// It finds all items with status=PENDING and next_retry_at <= now, ordered by created_at ASC (FIFO).
//
// Returns the number of successfully processed items and any critical error.
// Individual item failures are logged but don't stop batch processing.
func (w *QueueWorker) ProcessPendingItems(ctx context.Context) (int, error) {
	items, err := w.qr.FindPendingItems(ctx, w.batchSize)
	if err != nil {
		if errors.Is(err, ErrNoData) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to find pending items: %w", err)
	}

	processed := 0
	for i := range items {
		if err := w.processQueueItem(ctx, &items[i]); err != nil {
			w.logger.Errorf("Failed to process queue item %d: %v", items[i].ID, err)
			continue
		}
		processed++
	}

	return processed, nil
}

// ProcessRetryableItems processes failed items ready for retry attempts.
// It finds all items with status=FAILED and next_retry_at <= now, ordered by created_at ASC.
//
// Returns the number of successfully processed items and any critical error.
// Individual item failures are logged but don't stop batch processing.
func (w *QueueWorker) ProcessRetryableItems(ctx context.Context) (int, error) {
	items, err := w.qr.FindRetryableItems(ctx, w.batchSize)
	if err != nil {
		if errors.Is(err, ErrNoData) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to find retryable items: %w", err)
	}

	processed := 0
	for i := range items {
		if err := w.processQueueItem(ctx, &items[i]); err != nil {
			w.logger.Errorf("Failed to process retryable item %d: %v", items[i].ID, err)
			continue
		}
		processed++
	}

	return processed, nil
}

// processQueueItem processes a single queue item with retry logic.
func (w *QueueWorker) processQueueItem(ctx context.Context, queueItem *model.Queue) error {
	// Check if delivery can be attempted
	if err := queueItem.CanAttemptDelivery(w.retryStrategy.MaxAttempts); err != nil {
		w.logger.Debugf("Cannot attempt delivery for queue item %d: %v", queueItem.ID, err)
		return err
	}

	// Load subscription to get callback URL
	subscription, err := w.sr.Load(ctx, queueItem.SubscriptionID)
	if err != nil {
		return fmt.Errorf("failed to load subscription: %w", err)
	}

	// Load message
	message, err := w.mr.Load(ctx, queueItem.MessageID)
	if err != nil {
		return fmt.Errorf("failed to load message: %w", err)
	}

	// Prepare message for delivery
	dataMessage, err := w.prepareMessage(message)
	if err != nil {
		return fmt.Errorf("failed to prepare message: %w", err)
	}

	// Get callback URL via transmitter provider (avoiding circular dependency)
	callbackURL, err := w.transmitterProvider.GetCallbackUrl(ctx, subscription.SubscriberID)
	if err != nil {
		return fmt.Errorf("failed to get callback URL: %w", err)
	}

	// Attempt delivery using the gateway interface
	err = w.gateway.DeliverMessage(ctx, callbackURL, dataMessage)
	if err != nil {
		// Delivery failed
		w.handleDeliveryFailure(ctx, queueItem, err)
		return fmt.Errorf("delivery failed: %w", err)
	}

	// Delivery succeeded
	w.handleDeliverySuccess(ctx, queueItem)
	return nil
}

// prepareMessage prepares a message for delivery.
func (w *QueueWorker) prepareMessage(message model.Message) (*model.DataMessage, error) {
	strBase64 := base64.StdEncoding.EncodeToString([]byte(message.Data))

	dataMessage := model.NewDataMessage(
		fmt.Sprintf("%d", message.ID),
		message.CreatedAt,
		message.Identifier,
		strBase64,
	)

	if err := dataMessage.FromString(message.Data); err != nil {
		return nil, fmt.Errorf("failed to parse message data: %w", err)
	}

	return dataMessage, nil
}

// handleDeliverySuccess handles successful message delivery.
func (w *QueueWorker) handleDeliverySuccess(ctx context.Context, queueItem *model.Queue) {
	queueItem.MarkSent()

	if _, err := w.qr.Save(ctx, queueItem); err != nil {
		w.logger.Errorf("Failed to mark queue item %d as sent: %v", queueItem.ID, err)
		return
	}

	w.logger.Infof("Successfully delivered message %d (queue_id=%d, attempts=%d)",
		queueItem.MessageID, queueItem.ID, queueItem.AttemptCount)
}

// handleDeliveryFailure handles failed message delivery with retry logic.
func (w *QueueWorker) handleDeliveryFailure(ctx context.Context, queueItem *model.Queue, deliveryErr error) {
	// Calculate next retry delay
	retryDelay := w.retryStrategy.CalculateRetryDelay(queueItem.AttemptCount + 1)

	// Mark as failed with retry schedule
	queueItem.MarkFailed(deliveryErr, retryDelay)

	if _, err := w.qr.Save(ctx, queueItem); err != nil {
		w.logger.Errorf("Failed to update queue item %d after failure: %v", queueItem.ID, err)
		return
	}

	// Notify about delivery failure
	if err := w.notificationService.NotifyDeliveryFailure(ctx, queueItem, deliveryErr); err != nil {
		w.logger.Warnf("Failed to send delivery failure notification: %v", err)
	}

	// Check if should move to DLQ
	if queueItem.ShouldMoveToDLQ(w.retryStrategy.DLQThreshold) {
		w.logger.Warnf("Moving queue item %d to DLQ (attempts=%d, threshold=%d)",
			queueItem.ID, queueItem.AttemptCount, w.retryStrategy.DLQThreshold)

		// Move to DLQ
		if err := w.moveToDLQ(ctx, queueItem, deliveryErr); err != nil {
			w.logger.Errorf("Failed to move queue item %d to DLQ: %v", queueItem.ID, err)
		}
		return
	}

	w.logger.Warnf("Delivery failed for message %d (queue_id=%d, attempts=%d, next_retry=%v): %v",
		queueItem.MessageID, queueItem.ID, queueItem.AttemptCount, retryDelay, deliveryErr)
}

// CleanupExpiredItems removes expired queue items from the queue.
// Items are considered expired when expires_at <= now and status != SENT.
//
// This prevents the queue from growing indefinitely with stale messages.
// Returns the number of deleted items and any critical error.
func (w *QueueWorker) CleanupExpiredItems(ctx context.Context) (int, error) {
	items, err := w.qr.FindExpiredItems(ctx, w.batchSize)
	if err != nil {
		if errors.Is(err, ErrNoData) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to find expired items: %w", err)
	}

	deleted := 0
	for i := range items {
		if err := w.qr.Delete(ctx, &items[i]); err != nil {
			w.logger.Errorf("Failed to delete expired queue item %d: %v", items[i].ID, err)
			continue
		}
		deleted++
	}

	w.logger.Infof("Cleaned up %d expired queue items", deleted)
	return deleted, nil
}

// Run starts the queue worker event loop that processes messages continuously.
// It runs until the context is canceled, processing batches at the specified interval.
//
// Each batch processes:
//   - Pending items (first delivery attempt)
//   - Retryable items (retry after backoff delay)
//   - Expired items (cleanup)
//
// This method blocks and should typically be run in a goroutine.
//
// Example:
//
//	ctx := context.Background()
//	go worker.Run(ctx, 30*time.Second) // Process every 30 seconds
func (w *QueueWorker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w.logger.Info("Queue worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Queue worker stopped")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

// processBatch processes one batch of pending and retryable items.
func (w *QueueWorker) processBatch(ctx context.Context) {
	// Process pending items (first delivery)
	pendingCount, err := w.ProcessPendingItems(ctx)
	if err != nil {
		w.logger.Errorf("Error processing pending items: %v", err)
	}

	// Process retryable items (retry attempts)
	retryCount, err := w.ProcessRetryableItems(ctx)
	if err != nil {
		w.logger.Errorf("Error processing retryable items: %v", err)
	}

	// Periodic cleanup of expired items
	expiredCount, err := w.CleanupExpiredItems(ctx)
	if err != nil {
		w.logger.Errorf("Error cleaning up expired items: %v", err)
	}

	if pendingCount > 0 || retryCount > 0 || expiredCount > 0 {
		w.logger.Infof("Batch processed: pending=%d, retries=%d, expired=%d",
			pendingCount, retryCount, expiredCount)
	}
}

// GetRetrySchedule returns a human-readable description of the retry schedule.
// Useful for displaying retry configuration to users or in logs.
//
// Example output: "30s → 1m → 2m → 4m → 8m → 16m → 30m".
func (w *QueueWorker) GetRetrySchedule() string {
	return w.retryStrategy.GetRetrySchedule()
}

// moveToDLQ moves a failed queue item to the Dead Letter Queue after retry exhaustion.
// It creates a DLQ entry with full diagnostic information and removes the item from the queue.
//
// This method is called automatically when a queue item exceeds the retry threshold.
func (w *QueueWorker) moveToDLQ(ctx context.Context, queueItem *model.Queue, _ error) error {
	// Load message for DLQ entry
	message, err := w.mr.Load(ctx, queueItem.MessageID)
	if err != nil {
		return fmt.Errorf("failed to load message for DLQ: %w", err)
	}

	// Load subscription
	subscription, err := w.sr.Load(ctx, queueItem.SubscriptionID)
	if err != nil {
		return fmt.Errorf("failed to load subscription for DLQ: %w", err)
	}

	// Get callback URL
	callbackURL, err := w.transmitterProvider.GetCallbackUrl(ctx, subscription.SubscriberID)
	if err != nil {
		w.logger.Warnf("Failed to get callback URL for DLQ entry: %v", err)
		callbackURL = "unknown" // Still create DLQ entry with placeholder
	}

	// Determine failure reason
	failureReason := fmt.Sprintf("Max retry attempts exceeded (%d >= %d)",
		queueItem.AttemptCount, w.retryStrategy.DLQThreshold)

	// Create DLQ entry
	dlqEntry := model.NewDeadLetterQueue(
		queueItem.SubscriptionID,
		queueItem.MessageID,
		queueItem.ID,
		queueItem.AttemptCount,
		queueItem.LastError.String,
		failureReason,
		queueItem.CreatedAt,          // First attempt
		queueItem.LastAttemptAt.Time, // Last attempt
		message.Data,                 // Message payload
		callbackURL,                  // Target URL
	)

	// Save DLQ entry
	_, err = w.dlqr.Save(ctx, dlqEntry)
	if err != nil {
		return fmt.Errorf("failed to save DLQ entry: %w", err)
	}

	// Delete from queue (moved to DLQ)
	if err := w.qr.Delete(ctx, queueItem); err != nil {
		w.logger.Errorf("Failed to delete queue item %d after moving to DLQ: %v", queueItem.ID, err)
		// Don't return error - DLQ entry is already created
	}

	w.logger.Infof("Moved message %d to DLQ (queue_id=%d, dlq_id=%d, attempts=%d, reason=%s)",
		queueItem.MessageID, queueItem.ID, dlqEntry.ID, queueItem.AttemptCount, failureReason)

	// Notify about DLQ item
	if err := w.notificationService.NotifyDLQItemAdded(ctx, dlqEntry); err != nil {
		w.logger.Warnf("Failed to send DLQ notification: %v", err)
	}

	return nil
}

// GetDLQStats retrieves Dead Letter Queue statistics for monitoring.
// Returns aggregated stats including total count, unresolved count, resolution rate, and average age.
//
// Useful for dashboards, monitoring systems, and operational visibility.
func (w *QueueWorker) GetDLQStats(ctx context.Context) (model.DLQStats, error) {
	return w.dlqr.GetStats(ctx)
}
