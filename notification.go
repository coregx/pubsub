package pubsub

import (
	"context"

	"github.com/coregx/pubsub/model"
)

// NotificationService defines an optional interface for sending notifications
// about pub/sub system events (failures, DLQ items, etc.).
//
// Implementations might send emails, Slack messages, SMS, or log to monitoring systems.
type NotificationService interface {
	// NotifyDLQItemAdded is called when a message is moved to the Dead Letter Queue.
	// This indicates a message failed after all retry attempts.
	NotifyDLQItemAdded(ctx context.Context, dlq model.DeadLetterQueue) error

	// NotifyDeliveryFailure is called when a message delivery fails.
	// This is informational and happens before moving to DLQ.
	NotifyDeliveryFailure(ctx context.Context, queue *model.Queue, err error) error

	// NotifySubscriptionCreated is called when a new subscription is created.
	NotifySubscriptionCreated(ctx context.Context, subscription model.Subscription) error

	// NotifySubscriptionDeactivated is called when a subscription is deactivated.
	NotifySubscriptionDeactivated(ctx context.Context, subscription model.Subscription) error
}

// NoOpNotificationService is a no-op implementation of NotificationService.
// Use this when notifications are not needed.
type NoOpNotificationService struct{}

// NotifyDLQItemAdded does nothing.
func (n *NoOpNotificationService) NotifyDLQItemAdded(_ context.Context, _ model.DeadLetterQueue) error {
	return nil
}

// NotifyDeliveryFailure does nothing.
func (n *NoOpNotificationService) NotifyDeliveryFailure(_ context.Context, _ *model.Queue, _ error) error {
	return nil
}

// NotifySubscriptionCreated does nothing.
func (n *NoOpNotificationService) NotifySubscriptionCreated(_ context.Context, _ model.Subscription) error {
	return nil
}

// NotifySubscriptionDeactivated does nothing.
func (n *NoOpNotificationService) NotifySubscriptionDeactivated(_ context.Context, _ model.Subscription) error {
	return nil
}

// LoggingNotificationService is a simple implementation that logs notifications.
type LoggingNotificationService struct {
	logger Logger
}

// NewLoggingNotificationService creates a new LoggingNotificationService.
func NewLoggingNotificationService(logger Logger) *LoggingNotificationService {
	return &LoggingNotificationService{logger: logger}
}

// NotifyDLQItemAdded logs DLQ item addition.
func (n *LoggingNotificationService) NotifyDLQItemAdded(_ context.Context, dlq model.DeadLetterQueue) error {
	n.logger.Warnf("⚠️ Message moved to DLQ: message_id=%d, subscription_id=%d, attempts=%d, reason=%s",
		dlq.MessageID, dlq.SubscriptionID, dlq.AttemptCount, dlq.FailureReason)
	return nil
}

// NotifyDeliveryFailure logs delivery failure.
func (n *LoggingNotificationService) NotifyDeliveryFailure(_ context.Context, queue *model.Queue, err error) error {
	n.logger.Warnf("⚠️ Delivery failed: queue_id=%d, message_id=%d, attempt=%d, error=%v",
		queue.ID, queue.MessageID, queue.AttemptCount, err)
	return nil
}

// NotifySubscriptionCreated logs subscription creation.
func (n *LoggingNotificationService) NotifySubscriptionCreated(_ context.Context, subscription model.Subscription) error {
	n.logger.Infof("✅ Subscription created: id=%d, subscriber_id=%d, topic_id=%d, identifier=%s",
		subscription.ID, subscription.SubscriberID, subscription.TopicID, subscription.Identifier)
	return nil
}

// NotifySubscriptionDeactivated logs subscription deactivation.
func (n *LoggingNotificationService) NotifySubscriptionDeactivated(_ context.Context, subscription model.Subscription) error {
	n.logger.Infof("🔴 Subscription deactivated: id=%d, subscriber_id=%d",
		subscription.ID, subscription.SubscriberID)
	return nil
}
