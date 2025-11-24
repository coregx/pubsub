package pubsub

import "context"

// NotificationSender defines interface for sending notifications
// This allows different implementations (email, SMS, etc.) to be plugged in
type NotificationSender interface {
	// SendAdminNotification sends a notification to administrators
	SendAdminNotification(ctx context.Context, notification AdminNotification) error
}

// AdminNotification represents an administrative notification
type AdminNotification struct {
	To         string
	Subject    string
	Body       string
	SmsMessage string
	Priority   string // "high", "medium", "low"
}

// NoopNotificationSender is a no-op implementation for testing
type NoopNotificationSender struct{}

func (n *NoopNotificationSender) SendAdminNotification(ctx context.Context, notification AdminNotification) error {
	// No-op for testing or when notifications are disabled
	return nil
}
