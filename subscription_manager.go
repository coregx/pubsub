package pubsub

import (
	"context"
	"fmt"

	"github.com/coregx/pubsub/model"
)

// SubscriptionManager handles subscription lifecycle management for the pub/sub system.
// It provides high-level operations for creating, managing, and querying subscriptions
// that connect subscribers to topics.
//
// Key operations:
//   - Subscribe: Create new subscriptions with validation
//   - Unsubscribe: Deactivate existing subscriptions
//   - ListSubscriptions: Query subscriptions by subscriber and identifier
//   - ReactivateSubscription: Re-enable previously deactivated subscriptions
//
// Thread safety: Safe for concurrent use.
type SubscriptionManager struct {
	subscriptionRepo SubscriptionRepository
	subscriberRepo   SubscriberRepository
	topicRepo        TopicRepository
	logger           Logger
}

// SubscriptionManagerOption is a function that configures a SubscriptionManager.
// Used with the Options Pattern for flexible service construction.
type SubscriptionManagerOption func(*SubscriptionManager) error

// NewSubscriptionManager creates a new SubscriptionManager with the provided options.
//
// Required options:
//   - WithSubscriptionManagerRepositories: subscription, subscriber, and topic repositories
//   - WithSubscriptionManagerLogger: logger instance
//
// Example:
//
//	manager, err := pubsub.NewSubscriptionManager(
//	    pubsub.WithSubscriptionManagerRepositories(subRepo, subscriberRepo, topicRepo),
//	    pubsub.WithSubscriptionManagerLogger(logger),
//	)
func NewSubscriptionManager(opts ...SubscriptionManagerOption) (*SubscriptionManager, error) {
	sm := &SubscriptionManager{}

	for _, opt := range opts {
		if err := opt(sm); err != nil {
			return nil, NewErrorWithCause(ErrCodeConfiguration, "failed to apply subscription manager option", err)
		}
	}

	// Validate required dependencies
	if sm.subscriptionRepo == nil {
		return nil, NewError(ErrCodeConfiguration, "SubscriptionRepository is required")
	}
	if sm.subscriberRepo == nil {
		return nil, NewError(ErrCodeConfiguration, "SubscriberRepository is required")
	}
	if sm.topicRepo == nil {
		return nil, NewError(ErrCodeConfiguration, "TopicRepository is required")
	}
	if sm.logger == nil {
		return nil, NewError(ErrCodeConfiguration, "Logger is required")
	}

	return sm, nil
}

// WithSubscriptionManagerRepositories sets the required repository dependencies
// for the subscription manager. All repositories are required and must not be nil.
//
// This is a required option for NewSubscriptionManager.
func WithSubscriptionManagerRepositories(
	subscriptionRepo SubscriptionRepository,
	subscriberRepo SubscriberRepository,
	topicRepo TopicRepository,
) SubscriptionManagerOption {
	return func(sm *SubscriptionManager) error {
		if subscriptionRepo == nil {
			return fmt.Errorf("subscriptionRepo cannot be nil")
		}
		if subscriberRepo == nil {
			return fmt.Errorf("subscriberRepo cannot be nil")
		}
		if topicRepo == nil {
			return fmt.Errorf("topicRepo cannot be nil")
		}

		sm.subscriptionRepo = subscriptionRepo
		sm.subscriberRepo = subscriberRepo
		sm.topicRepo = topicRepo
		return nil
	}
}

// WithSubscriptionManagerLogger sets the logger instance for the subscription manager.
// Logger is required and must not be nil.
//
// This is a required option for NewSubscriptionManager.
func WithSubscriptionManagerLogger(logger Logger) SubscriptionManagerOption {
	return func(sm *SubscriptionManager) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		sm.logger = logger
		return nil
	}
}

// SubscribeRequest represents a request to create a new subscription.
// All fields except CallbackURL are required.
type SubscribeRequest struct {
	SubscriberID int64  // ID of the subscriber (required, must exist)
	TopicCode    string // Topic code to subscribe to (required, must exist)
	Identifier   string // Event identifier filter (required, e.g., "user-123")
	CallbackURL  string // Webhook URL for message delivery (optional, can be set on subscriber)
}

// Subscribe creates a new subscription connecting a subscriber to a topic.
// It validates that both the subscriber and topic exist before creating the subscription.
// If an active subscription already exists, returns the existing subscription.
//
// Validation:
//   - SubscriberID must be > 0 and exist in database
//   - TopicCode must not be empty and exist in database
//   - Identifier must not be empty
//
// Returns the created (or existing) subscription, or an error if validation fails.
func (sm *SubscriptionManager) Subscribe(ctx context.Context, req SubscribeRequest) (*model.Subscription, error) {
	// Validate request
	if req.SubscriberID == 0 {
		return nil, NewError(ErrCodeValidation, "subscriber ID is required")
	}
	if req.TopicCode == "" {
		return nil, NewError(ErrCodeValidation, "topic code is required")
	}
	if req.Identifier == "" {
		return nil, NewError(ErrCodeValidation, "identifier is required")
	}

	// Validate subscriber exists
	_, err := sm.subscriberRepo.Load(ctx, req.SubscriberID)
	if err != nil {
		if IsNoData(err) {
			return nil, NewErrorWithCause(ErrCodeValidation, fmt.Sprintf("subscriber not found: %d", req.SubscriberID), err)
		}
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to load subscriber", err)
	}

	// Find topic by code
	topic, err := sm.topicRepo.GetByTopicCode(ctx, req.TopicCode)
	if err != nil {
		if IsNoData(err) {
			return nil, NewErrorWithCause(ErrCodeValidation, fmt.Sprintf("topic not found: %s", req.TopicCode), err)
		}
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to load topic", err)
	}

	// Check if subscription already exists
	existing, err := sm.subscriptionRepo.FindActive(ctx, req.SubscriberID, req.Identifier)
	if err != nil && !IsNoData(err) {
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to check existing subscriptions", err)
	}

	// Check for duplicate active subscription
	for _, sub := range existing {
		if sub.TopicID == topic.ID && sub.IsActive {
			sm.logger.Warnf("Subscription already exists: subscriber=%d, topic=%s, identifier=%s",
				req.SubscriberID, req.TopicCode, req.Identifier)
			return &sub, nil
		}
	}

	// Create new subscription
	subscription := model.NewSubscription(req.SubscriberID, topic.ID, req.Identifier, req.CallbackURL)
	subscription, err = sm.subscriptionRepo.Save(ctx, subscription)
	if err != nil {
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to save subscription", err)
	}

	sm.logger.Infof("Subscription created: id=%d, subscriber=%d, topic=%s, identifier=%s",
		subscription.ID, req.SubscriberID, req.TopicCode, req.Identifier)

	return &subscription, nil
}

// Unsubscribe deactivates an existing subscription.
// This is a soft delete - the subscription record remains in the database but becomes inactive.
// If the subscription is already inactive, returns the subscription without error.
//
// The subscription can be reactivated later using ReactivateSubscription.
//
// Returns the deactivated subscription or error if operation fails.
func (sm *SubscriptionManager) Unsubscribe(ctx context.Context, subscriptionID int64) (*model.Subscription, error) {
	if subscriptionID == 0 {
		return nil, NewError(ErrCodeValidation, "subscription ID is required")
	}

	// Load subscription
	subscription, err := sm.subscriptionRepo.Load(ctx, subscriptionID)
	if err != nil {
		if IsNoData(err) {
			return nil, NewErrorWithCause(ErrCodeValidation, fmt.Sprintf("subscription not found: %d", subscriptionID), err)
		}
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to load subscription", err)
	}

	// Check if already inactive
	if !subscription.IsActive {
		sm.logger.Warnf("Subscription already inactive: id=%d", subscriptionID)
		return &subscription, nil
	}

	// Deactivate subscription
	subscription.Deactivate()
	subscription, err = sm.subscriptionRepo.Save(ctx, subscription)
	if err != nil {
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to save subscription", err)
	}

	sm.logger.Infof("Subscription deactivated: id=%d", subscriptionID)

	return &subscription, nil
}

// ListSubscriptions returns all active subscriptions for a subscriber.
// Optionally filters by event identifier if provided.
//
// Parameters:
//   - subscriberID: Required, must be > 0
//   - identifier: Optional filter for event type (empty string = no filter)
//
// Returns empty slice if no subscriptions found (not an error).
func (sm *SubscriptionManager) ListSubscriptions(ctx context.Context, subscriberID int64, identifier string) ([]model.Subscription, error) {
	if subscriberID == 0 {
		return nil, NewError(ErrCodeValidation, "subscriber ID is required")
	}

	subscriptions, err := sm.subscriptionRepo.FindActive(ctx, subscriberID, identifier)
	if err != nil {
		if IsNoData(err) {
			return []model.Subscription{}, nil
		}
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to load subscriptions", err)
	}

	return subscriptions, nil
}

// GetSubscription retrieves a single subscription by ID.
// Returns the subscription or error if not found.
func (sm *SubscriptionManager) GetSubscription(ctx context.Context, subscriptionID int64) (*model.Subscription, error) {
	if subscriptionID == 0 {
		return nil, NewError(ErrCodeValidation, "subscription ID is required")
	}

	subscription, err := sm.subscriptionRepo.Load(ctx, subscriptionID)
	if err != nil {
		if IsNoData(err) {
			return nil, NewErrorWithCause(ErrCodeValidation, fmt.Sprintf("subscription not found: %d", subscriptionID), err)
		}
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to load subscription", err)
	}

	return &subscription, nil
}

// ReactivateSubscription reactivates a previously deactivated subscription.
// If the subscription is already active, returns without error.
//
// This allows resuming message delivery to a subscriber that was temporarily unsubscribed.
func (sm *SubscriptionManager) ReactivateSubscription(ctx context.Context, subscriptionID int64) (*model.Subscription, error) {
	if subscriptionID == 0 {
		return nil, NewError(ErrCodeValidation, "subscription ID is required")
	}

	// Load subscription
	subscription, err := sm.subscriptionRepo.Load(ctx, subscriptionID)
	if err != nil {
		if IsNoData(err) {
			return nil, NewErrorWithCause(ErrCodeValidation, fmt.Sprintf("subscription not found: %d", subscriptionID), err)
		}
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to load subscription", err)
	}

	// Check if already active
	if subscription.IsActive {
		sm.logger.Warnf("Subscription already active: id=%d", subscriptionID)
		return &subscription, nil
	}

	// Reactivate subscription
	subscription.IsActive = true
	subscription.DeletedAt.Valid = false
	subscription, err = sm.subscriptionRepo.Save(ctx, subscription)
	if err != nil {
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to save subscription", err)
	}

	sm.logger.Infof("Subscription reactivated: id=%d", subscriptionID)

	return &subscription, nil
}
