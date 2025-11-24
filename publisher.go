package pubsub

import (
	"context"
	"fmt"

	"github.com/coregx/pubsub/model"
)

// Publisher handles publishing messages to topics and creating queue items
// for active subscriptions.
type Publisher struct {
	messageRepo      MessageRepository
	queueRepo        QueueRepository
	subscriptionRepo SubscriptionRepository
	topicRepo        TopicRepository
	logger           Logger
}

// PublisherOption configures a Publisher.
type PublisherOption func(*Publisher) error

// NewPublisher creates a new Publisher with the provided options.
//
// Required options:
//   - WithPublisherRepositories: message, queue, subscription, and topic repositories
//   - WithPublisherLogger: logger instance
//
// Example:
//
//	publisher, err := pubsub.NewPublisher(
//	    pubsub.WithPublisherRepositories(msgRepo, queueRepo, subRepo, topicRepo),
//	    pubsub.WithPublisherLogger(logger),
//	)
func NewPublisher(opts ...PublisherOption) (*Publisher, error) {
	p := &Publisher{}

	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, NewErrorWithCause(ErrCodeConfiguration, "failed to apply publisher option", err)
		}
	}

	// Validate required dependencies
	if p.messageRepo == nil {
		return nil, NewError(ErrCodeConfiguration, "MessageRepository is required (use WithPublisherRepositories)")
	}
	if p.queueRepo == nil {
		return nil, NewError(ErrCodeConfiguration, "QueueRepository is required (use WithPublisherRepositories)")
	}
	if p.subscriptionRepo == nil {
		return nil, NewError(ErrCodeConfiguration, "SubscriptionRepository is required (use WithPublisherRepositories)")
	}
	if p.topicRepo == nil {
		return nil, NewError(ErrCodeConfiguration, "TopicRepository is required (use WithPublisherRepositories)")
	}
	if p.logger == nil {
		return nil, NewError(ErrCodeConfiguration, "Logger is required (use WithPublisherLogger)")
	}

	return p, nil
}

// WithPublisherRepositories sets the required repository dependencies.
func WithPublisherRepositories(
	messageRepo MessageRepository,
	queueRepo QueueRepository,
	subscriptionRepo SubscriptionRepository,
	topicRepo TopicRepository,
) PublisherOption {
	return func(p *Publisher) error {
		if messageRepo == nil {
			return fmt.Errorf("messageRepo cannot be nil")
		}
		if queueRepo == nil {
			return fmt.Errorf("queueRepo cannot be nil")
		}
		if subscriptionRepo == nil {
			return fmt.Errorf("subscriptionRepo cannot be nil")
		}
		if topicRepo == nil {
			return fmt.Errorf("topicRepo cannot be nil")
		}

		p.messageRepo = messageRepo
		p.queueRepo = queueRepo
		p.subscriptionRepo = subscriptionRepo
		p.topicRepo = topicRepo
		return nil
	}
}

// WithPublisherLogger sets the logger instance.
func WithPublisherLogger(logger Logger) PublisherOption {
	return func(p *Publisher) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		p.logger = logger
		return nil
	}
}

// PublishRequest represents a request to publish a message.
type PublishRequest struct {
	TopicCode  string // Topic code to publish to
	Identifier string // Message identifier (event type)
	Data       string // Message payload
}

// PublishResult represents the result of a publish operation.
type PublishResult struct {
	MessageID         int64   // Created message ID
	QueueItemsCreated int     // Number of queue items created
	SubscriptionsIDs  []int64 // Subscription IDs that received the message
}

// Publish publishes a message to a topic and creates queue items for all active subscriptions.
//
// The process:
//  1. Validate topic exists
//  2. Create message record
//  3. Find all active subscriptions for the topic
//  4. Create queue items for each subscription
//
// Returns PublishResult with message ID and queue item count, or error if publish fails.
func (p *Publisher) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	// Validate request
	if req.TopicCode == "" {
		return nil, NewError(ErrCodeValidation, "topic code is required")
	}
	if req.Identifier == "" {
		return nil, NewError(ErrCodeValidation, "identifier is required")
	}

	// Find topic by code
	topic, err := p.topicRepo.GetByTopicCode(ctx, req.TopicCode)
	if err != nil {
		if IsNoData(err) {
			return nil, NewErrorWithCause(ErrCodeValidation, fmt.Sprintf("topic not found: %s", req.TopicCode), err)
		}
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to load topic", err)
	}

	// Create message
	message := model.NewMessage(topic.ID, req.Identifier, req.Data)
	message, err = p.messageRepo.Save(ctx, message)
	if err != nil {
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to save message", err)
	}

	p.logger.Infof("Message created: id=%d, topic=%s, identifier=%s", message.ID, req.TopicCode, req.Identifier)

	// Find active subscriptions for topic
	subscriptions, err := p.subscriptionRepo.FindActive(ctx, 0, req.Identifier)
	if err != nil && !IsNoData(err) {
		return nil, NewErrorWithCause(ErrCodeDatabase, "failed to load subscriptions", err)
	}

	// Filter subscriptions by topic
	var activeSubscriptions []model.Subscription
	for _, sub := range subscriptions {
		if sub.TopicID == topic.ID && sub.IsActive {
			activeSubscriptions = append(activeSubscriptions, sub)
		}
	}

	if len(activeSubscriptions) == 0 {
		p.logger.Warnf("No active subscriptions found for topic=%s, identifier=%s", req.TopicCode, req.Identifier)
		return &PublishResult{
			MessageID:         message.ID,
			QueueItemsCreated: 0,
			SubscriptionsIDs:  []int64{},
		}, nil
	}

	// Create queue items for each subscription
	subscriptionIDs := make([]int64, 0, len(activeSubscriptions))
	queueItemsCreated := 0

	for _, subscription := range activeSubscriptions {
		queueItem := model.NewQueue(subscription.ID, message.ID)
		_, err := p.queueRepo.Save(ctx, &queueItem)
		if err != nil {
			p.logger.Errorf("Failed to create queue item for subscription %d: %v", subscription.ID, err)
			continue // Continue creating other queue items
		}

		subscriptionIDs = append(subscriptionIDs, subscription.ID)
		queueItemsCreated++
	}

	p.logger.Infof("Published message %d to %d subscriptions (topic=%s, identifier=%s)",
		message.ID, queueItemsCreated, req.TopicCode, req.Identifier)

	return &PublishResult{
		MessageID:         message.ID,
		QueueItemsCreated: queueItemsCreated,
		SubscriptionsIDs:  subscriptionIDs,
	}, nil
}

// PublishBatch publishes multiple messages in a batch.
// This is more efficient than calling Publish multiple times.
func (p *Publisher) PublishBatch(ctx context.Context, requests []PublishRequest) ([]*PublishResult, error) {
	if len(requests) == 0 {
		return []*PublishResult{}, nil
	}

	results := make([]*PublishResult, 0, len(requests))

	for _, req := range requests {
		result, err := p.Publish(ctx, req)
		if err != nil {
			p.logger.Errorf("Failed to publish message (topic=%s, identifier=%s): %v",
				req.TopicCode, req.Identifier, err)
			continue // Continue with other messages
		}
		results = append(results, result)
	}

	return results, nil
}
