package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coregx/pubsub/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMessageRepository implements MessageRepository for testing.
type mockMessageRepository struct {
	messages map[int64]model.Message
	nextID   int64
	saveErr  error
	loadErr  error
}

func newMockMessageRepository() *mockMessageRepository {
	return &mockMessageRepository{
		messages: make(map[int64]model.Message),
		nextID:   1,
	}
}

func (m *mockMessageRepository) Load(_ context.Context, id int64) (model.Message, error) {
	if m.loadErr != nil {
		return model.Message{}, m.loadErr
	}
	if msg, ok := m.messages[id]; ok {
		return msg, nil
	}
	return model.Message{}, ErrNoData
}

func (m *mockMessageRepository) Save(_ context.Context, msg model.Message) (model.Message, error) {
	if m.saveErr != nil {
		return model.Message{}, m.saveErr
	}
	if msg.ID == 0 {
		msg.ID = m.nextID
		m.nextID++
	}
	m.messages[msg.ID] = msg
	return msg, nil
}

func (m *mockMessageRepository) Delete(_ context.Context, _ model.Message) error {
	return nil
}

func (m *mockMessageRepository) FindOutdatedMessages(_ context.Context, _ int) ([]model.Message, error) {
	return nil, ErrNoData
}

// mockQueueRepositoryForPublisher implements QueueRepository for Publisher tests.
type mockQueueRepositoryForPublisher struct {
	items   map[int64]*model.Queue
	nextID  int64
	saveErr error
}

func newMockQueueRepositoryForPublisher() *mockQueueRepositoryForPublisher {
	return &mockQueueRepositoryForPublisher{
		items:  make(map[int64]*model.Queue),
		nextID: 1,
	}
}

func (m *mockQueueRepositoryForPublisher) Load(_ context.Context, id int64) (model.Queue, error) {
	if item, ok := m.items[id]; ok {
		return *item, nil
	}
	return model.Queue{}, ErrNoData
}

func (m *mockQueueRepositoryForPublisher) Save(_ context.Context, q *model.Queue) (*model.Queue, error) {
	if m.saveErr != nil {
		return nil, m.saveErr
	}
	if q.ID == 0 {
		q.ID = m.nextID
		m.nextID++
	}
	saved := *q
	m.items[q.ID] = &saved
	return &saved, nil
}

func (m *mockQueueRepositoryForPublisher) Delete(_ context.Context, _ *model.Queue) error {
	return nil
}

func (m *mockQueueRepositoryForPublisher) FindByMessageID(_ context.Context, _, _ int64) (model.Queue, error) {
	return model.Queue{}, ErrNoData
}

func (m *mockQueueRepositoryForPublisher) FindBySubscriptionID(_ context.Context, _ int64) ([]model.Queue, error) {
	return nil, ErrNoData
}

func (m *mockQueueRepositoryForPublisher) FindPendingItems(_ context.Context, _ int) ([]model.Queue, error) {
	return nil, ErrNoData
}

func (m *mockQueueRepositoryForPublisher) FindRetryableItems(_ context.Context, _ int) ([]model.Queue, error) {
	return nil, ErrNoData
}

func (m *mockQueueRepositoryForPublisher) FindExpiredItems(_ context.Context, _ int) ([]model.Queue, error) {
	return nil, ErrNoData
}

func (m *mockQueueRepositoryForPublisher) UpdateNextRetry(_ context.Context, _ int64, _ time.Time, _ int) error {
	return nil
}

// mockSubscriptionRepositoryForPublisher implements SubscriptionRepository for Publisher tests.
type mockSubscriptionRepositoryForPublisher struct {
	subscriptions map[int64]model.Subscription
	active        []model.Subscription
	findActiveErr error
}

func newMockSubscriptionRepositoryForPublisher() *mockSubscriptionRepositoryForPublisher {
	return &mockSubscriptionRepositoryForPublisher{
		subscriptions: make(map[int64]model.Subscription),
	}
}

func (m *mockSubscriptionRepositoryForPublisher) Load(_ context.Context, id int64) (model.Subscription, error) {
	if sub, ok := m.subscriptions[id]; ok {
		return sub, nil
	}
	return model.Subscription{}, ErrNoData
}

func (m *mockSubscriptionRepositoryForPublisher) Save(_ context.Context, sub model.Subscription) (model.Subscription, error) {
	m.subscriptions[sub.ID] = sub
	return sub, nil
}

func (m *mockSubscriptionRepositoryForPublisher) FindActive(_ context.Context, _ int64, _ string) ([]model.Subscription, error) {
	if m.findActiveErr != nil {
		return nil, m.findActiveErr
	}
	if len(m.active) == 0 {
		return nil, ErrNoData
	}
	return m.active, nil
}

func (m *mockSubscriptionRepositoryForPublisher) List(_ context.Context, _ Filter) ([]model.Subscription, error) {
	return nil, ErrNoData
}

func (m *mockSubscriptionRepositoryForPublisher) FindAllActive(_ context.Context) ([]model.SubscriptionFull, error) {
	return nil, ErrNoData
}

// mockTopicRepository implements TopicRepository for testing.
type mockTopicRepository struct {
	topics         map[string]model.Topic
	getByCodeErr   error
}

func newMockTopicRepository() *mockTopicRepository {
	return &mockTopicRepository{
		topics: make(map[string]model.Topic),
	}
}

func (m *mockTopicRepository) Load(_ context.Context, id int64) (model.Topic, error) {
	for _, t := range m.topics {
		if t.ID == id {
			return t, nil
		}
	}
	return model.Topic{}, ErrNoData
}

func (m *mockTopicRepository) Save(_ context.Context, t model.Topic) (model.Topic, error) {
	m.topics[t.Code] = t
	return t, nil
}

func (m *mockTopicRepository) GetByTopicCode(_ context.Context, code string) (model.Topic, error) {
	if m.getByCodeErr != nil {
		return model.Topic{}, m.getByCodeErr
	}
	if t, ok := m.topics[code]; ok {
		return t, nil
	}
	return model.Topic{}, ErrNoData
}

// buildPublisher constructs a Publisher with the given repositories and a NoopLogger.
func buildPublisher(
	t *testing.T,
	msgRepo MessageRepository,
	queueRepo QueueRepository,
	subRepo SubscriptionRepository,
	topicRepo TopicRepository,
) *Publisher {
	t.Helper()
	p, err := NewPublisher(
		WithPublisherRepositories(msgRepo, queueRepo, subRepo, topicRepo),
		WithPublisherLogger(&NoopLogger{}),
	)
	require.NoError(t, err)
	return p
}

// TestNewPublisher_ValidOptions verifies a Publisher is created when all options are provided.
func TestNewPublisher_ValidOptions(t *testing.T) {
	p, err := NewPublisher(
		WithPublisherRepositories(
			newMockMessageRepository(),
			newMockQueueRepositoryForPublisher(),
			newMockSubscriptionRepositoryForPublisher(),
			newMockTopicRepository(),
		),
		WithPublisherLogger(&NoopLogger{}),
	)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

// TestNewPublisher_MissingMessageRepo verifies that a nil message repository returns an error.
func TestNewPublisher_MissingMessageRepo(t *testing.T) {
	_, err := NewPublisher(
		WithPublisherLogger(&NoopLogger{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MessageRepository is required")
}

// TestNewPublisher_MissingLogger verifies that a nil logger returns an error.
func TestNewPublisher_MissingLogger(t *testing.T) {
	_, err := NewPublisher(
		WithPublisherRepositories(
			newMockMessageRepository(),
			newMockQueueRepositoryForPublisher(),
			newMockSubscriptionRepositoryForPublisher(),
			newMockTopicRepository(),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Logger is required")
}

// TestNewPublisher_NilRepoInOption verifies that passing nil to WithPublisherRepositories returns an error.
func TestNewPublisher_NilRepoInOption(t *testing.T) {
	_, err := NewPublisher(
		WithPublisherRepositories(nil, nil, nil, nil),
		WithPublisherLogger(&NoopLogger{}),
	)
	require.Error(t, err)
}

// TestNewPublisher_NilLogger verifies that passing nil to WithPublisherLogger returns an error.
func TestNewPublisher_NilLogger(t *testing.T) {
	_, err := NewPublisher(
		WithPublisherRepositories(
			newMockMessageRepository(),
			newMockQueueRepositoryForPublisher(),
			newMockSubscriptionRepositoryForPublisher(),
			newMockTopicRepository(),
		),
		WithPublisherLogger(nil),
	)
	require.Error(t, err)
}

// TestPublish_EmptyTopicCode verifies that an empty topic code returns a validation error.
func TestPublish_EmptyTopicCode(t *testing.T) {
	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(),
		newMockTopicRepository(),
	)

	_, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "",
		Identifier: "user-123",
		Data:       `{"event":"test"}`,
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "topic code is required")
}

// TestPublish_EmptyIdentifier verifies that an empty identifier returns a validation error.
func TestPublish_EmptyIdentifier(t *testing.T) {
	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(),
		newMockTopicRepository(),
	)

	_, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "",
		Data:       `{"event":"test"}`,
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "identifier is required")
}

// TestPublish_TopicNotFound verifies that publishing to a nonexistent topic returns a validation error.
func TestPublish_TopicNotFound(t *testing.T) {
	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(),
		newMockTopicRepository(), // empty topic repository
	)

	_, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "unknown.topic",
		Identifier: "user-123",
		Data:       `{}`,
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "topic not found")
}

// TestPublish_TopicRepoError verifies that a database error from the topic repo propagates correctly.
func TestPublish_TopicRepoError(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.getByCodeErr = errors.New("database connection lost")

	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(),
		topicRepo,
	)

	_, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       `{}`,
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
	assert.Contains(t, pubErr.Message, "failed to load topic")
}

// TestPublish_MessageSaveError verifies that a message repository save error propagates correctly.
func TestPublish_MessageSaveError(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 1, Code: "user.signup", Name: "User Signup", IsActive: true}

	msgRepo := newMockMessageRepository()
	msgRepo.saveErr = errors.New("disk full")

	p := buildPublisher(t,
		msgRepo,
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(),
		topicRepo,
	)

	_, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       `{}`,
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
	assert.Contains(t, pubErr.Message, "failed to save message")
}

// TestPublish_NoActiveSubscriptions verifies that publishing to a topic with no subscriptions
// returns a successful result with zero queue items.
func TestPublish_NoActiveSubscriptions(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 1, Code: "user.signup", Name: "User Signup", IsActive: true}

	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(), // no active subs
		topicRepo,
	)

	result, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       `{"userId":42}`,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.MessageID, int64(0))
	assert.Equal(t, 0, result.QueueItemsCreated)
	assert.Empty(t, result.SubscriptionsIDs)
}

// TestPublish_WithActiveSubscriptions verifies that publishing creates queue items for each
// active subscription matching the topic.
func TestPublish_WithActiveSubscriptions(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	subRepo := newMockSubscriptionRepositoryForPublisher()
	subRepo.active = []model.Subscription{
		{ID: 1, SubscriberID: 100, TopicID: 10, Identifier: "user-123", IsActive: true},
		{ID: 2, SubscriberID: 101, TopicID: 10, Identifier: "user-123", IsActive: true},
	}

	queueRepo := newMockQueueRepositoryForPublisher()

	p := buildPublisher(t,
		newMockMessageRepository(),
		queueRepo,
		subRepo,
		topicRepo,
	)

	result, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       `{"userId":42}`,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.MessageID, int64(0))
	assert.Equal(t, 2, result.QueueItemsCreated)
	assert.Len(t, result.SubscriptionsIDs, 2)
	assert.Contains(t, result.SubscriptionsIDs, int64(1))
	assert.Contains(t, result.SubscriptionsIDs, int64(2))
}

// TestPublish_SubscriptionsFilteredByTopic verifies that subscriptions for different topics
// are not included in the queue item creation.
func TestPublish_SubscriptionsFilteredByTopic(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	subRepo := newMockSubscriptionRepositoryForPublisher()
	// TopicID 10 is our topic; TopicID 99 is a different topic
	subRepo.active = []model.Subscription{
		{ID: 1, SubscriberID: 100, TopicID: 10, Identifier: "user-123", IsActive: true},
		{ID: 2, SubscriberID: 101, TopicID: 99, Identifier: "user-123", IsActive: true}, // different topic
	}

	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		subRepo,
		topicRepo,
	)

	result, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       `{"userId":42}`,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.QueueItemsCreated)
	assert.Equal(t, []int64{1}, result.SubscriptionsIDs)
}

// TestPublish_InactiveSubscriptionSkipped verifies that inactive subscriptions are skipped.
func TestPublish_InactiveSubscriptionSkipped(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	subRepo := newMockSubscriptionRepositoryForPublisher()
	subRepo.active = []model.Subscription{
		{ID: 1, SubscriberID: 100, TopicID: 10, Identifier: "user-123", IsActive: false}, // inactive
	}

	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		subRepo,
		topicRepo,
	)

	result, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       `{}`,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.QueueItemsCreated)
}

// TestPublish_QueueSaveErrorContinues verifies that a queue save error does not abort publishing:
// remaining subscriptions still receive queue items.
func TestPublish_QueueSaveErrorContinues(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	subRepo := newMockSubscriptionRepositoryForPublisher()
	subRepo.active = []model.Subscription{
		{ID: 1, SubscriberID: 100, TopicID: 10, Identifier: "user-123", IsActive: true},
		{ID: 2, SubscriberID: 101, TopicID: 10, Identifier: "user-123", IsActive: true},
	}

	queueRepo := newMockQueueRepositoryForPublisher()
	queueRepo.saveErr = errors.New("queue table locked")

	p := buildPublisher(t,
		newMockMessageRepository(),
		queueRepo,
		subRepo,
		topicRepo,
	)

	result, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       `{}`,
	})

	// Publish itself should not fail - errors per queue item are logged and skipped
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.QueueItemsCreated)
}

// TestPublish_SubscriptionRepoError verifies that a database error from FindActive propagates.
func TestPublish_SubscriptionRepoError(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	subRepo := newMockSubscriptionRepositoryForPublisher()
	subRepo.findActiveErr = errors.New("subscription table unavailable")

	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		subRepo,
		topicRepo,
	)

	_, err := p.Publish(context.Background(), PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       `{}`,
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
}

// TestPublishBatch_EmptyRequests verifies that an empty batch returns an empty result slice.
func TestPublishBatch_EmptyRequests(t *testing.T) {
	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(),
		newMockTopicRepository(),
	)

	results, err := p.PublishBatch(context.Background(), []PublishRequest{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// TestPublishBatch_MultipleMessages verifies that a batch of valid messages is published.
func TestPublishBatch_MultipleMessages(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}
	topicRepo.topics["order.created"] = model.Topic{ID: 11, Code: "order.created", Name: "Order Created", IsActive: true}

	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(),
		topicRepo,
	)

	results, err := p.PublishBatch(context.Background(), []PublishRequest{
		{TopicCode: "user.signup", Identifier: "user-1", Data: `{}`},
		{TopicCode: "order.created", Identifier: "order-1", Data: `{}`},
	})

	require.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestPublishBatch_PartialFailureContinues verifies that failing messages in a batch
// do not abort the remaining messages.
func TestPublishBatch_PartialFailureContinues(t *testing.T) {
	topicRepo := newMockTopicRepository()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	p := buildPublisher(t,
		newMockMessageRepository(),
		newMockQueueRepositoryForPublisher(),
		newMockSubscriptionRepositoryForPublisher(),
		topicRepo,
	)

	results, err := p.PublishBatch(context.Background(), []PublishRequest{
		{TopicCode: "unknown.topic", Identifier: "id-1", Data: `{}`}, // will fail
		{TopicCode: "user.signup", Identifier: "id-2", Data: `{}`},   // will succeed
	})

	require.NoError(t, err)
	// Only the successful one is in results
	assert.Len(t, results, 1)
	assert.Greater(t, results[0].MessageID, int64(0))
}
