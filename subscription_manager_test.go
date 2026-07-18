package pubsub

import (
	"context"
	"errors"
	"testing"

	"github.com/coregx/pubsub/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Subscription repository mock ---

// mockSubscriptionRepository implements SubscriptionRepository for SubscriptionManager tests.
type mockSubscriptionRepository struct {
	subscriptions map[int64]model.Subscription
	nextID        int64
	loadErr       error
	saveErr       error
	findActiveErr error
	activeItems   []model.Subscription
}

func newMockSubscriptionRepository() *mockSubscriptionRepository {
	return &mockSubscriptionRepository{
		subscriptions: make(map[int64]model.Subscription),
		nextID:        1,
	}
}

func (m *mockSubscriptionRepository) Load(_ context.Context, id int64) (model.Subscription, error) {
	if m.loadErr != nil {
		return model.Subscription{}, m.loadErr
	}
	if sub, ok := m.subscriptions[id]; ok {
		return sub, nil
	}
	return model.Subscription{}, ErrNoData
}

func (m *mockSubscriptionRepository) Save(_ context.Context, sub model.Subscription) (model.Subscription, error) {
	if m.saveErr != nil {
		return model.Subscription{}, m.saveErr
	}
	if sub.ID == 0 {
		sub.ID = m.nextID
		m.nextID++
	}
	m.subscriptions[sub.ID] = sub
	return sub, nil
}

func (m *mockSubscriptionRepository) FindActive(_ context.Context, _ int64, _ string) ([]model.Subscription, error) {
	if m.findActiveErr != nil {
		return nil, m.findActiveErr
	}
	if m.activeItems != nil {
		return m.activeItems, nil
	}
	return nil, ErrNoData
}

func (m *mockSubscriptionRepository) List(_ context.Context, _ Filter) ([]model.Subscription, error) {
	result := make([]model.Subscription, 0, len(m.subscriptions))
	for _, sub := range m.subscriptions {
		result = append(result, sub)
	}
	return result, nil
}

func (m *mockSubscriptionRepository) FindAllActive(_ context.Context) ([]model.SubscriptionFull, error) {
	return nil, ErrNoData
}

// --- Subscriber repository mock ---

// mockSubscriberRepository implements SubscriberRepository for SubscriptionManager tests.
type mockSubscriberRepository struct {
	subscribers map[int64]model.Subscriber
	loadErr     error
}

func newMockSubscriberRepository() *mockSubscriberRepository {
	return &mockSubscriberRepository{
		subscribers: make(map[int64]model.Subscriber),
	}
}

func (m *mockSubscriberRepository) Load(_ context.Context, id int64) (model.Subscriber, error) {
	if m.loadErr != nil {
		return model.Subscriber{}, m.loadErr
	}
	if sub, ok := m.subscribers[id]; ok {
		return sub, nil
	}
	return model.Subscriber{}, ErrNoData
}

func (m *mockSubscriberRepository) Save(_ context.Context, sub model.Subscriber) (model.Subscriber, error) {
	m.subscribers[sub.ID] = sub
	return sub, nil
}

func (m *mockSubscriberRepository) FindByClientID(_ context.Context, _ int64) (model.Subscriber, error) {
	return model.Subscriber{}, ErrNoData
}

func (m *mockSubscriberRepository) FindByName(_ context.Context, _ string) (model.Subscriber, error) {
	return model.Subscriber{}, ErrNoData
}

// --- Topic repository mock for SubscriptionManager ---

// mockTopicRepositoryForManager implements TopicRepository for SubscriptionManager tests.
type mockTopicRepositoryForManager struct {
	topics       map[string]model.Topic
	getByCodeErr error
}

func newMockTopicRepositoryForManager() *mockTopicRepositoryForManager {
	return &mockTopicRepositoryForManager{
		topics: make(map[string]model.Topic),
	}
}

func (m *mockTopicRepositoryForManager) Load(_ context.Context, id int64) (model.Topic, error) {
	for _, t := range m.topics {
		if t.ID == id {
			return t, nil
		}
	}
	return model.Topic{}, ErrNoData
}

func (m *mockTopicRepositoryForManager) Save(_ context.Context, t model.Topic) (model.Topic, error) {
	m.topics[t.Code] = t
	return t, nil
}

func (m *mockTopicRepositoryForManager) GetByTopicCode(_ context.Context, code string) (model.Topic, error) {
	if m.getByCodeErr != nil {
		return model.Topic{}, m.getByCodeErr
	}
	if t, ok := m.topics[code]; ok {
		return t, nil
	}
	return model.Topic{}, ErrNoData
}

// --- Manager builder helper ---

// buildManager constructs a SubscriptionManager with all required mocks.
func buildManager(
	t *testing.T,
	subRepo SubscriptionRepository,
	subscriberRepo SubscriberRepository,
	topicRepo TopicRepository,
) *SubscriptionManager {
	t.Helper()
	m, err := NewSubscriptionManager(
		WithSubscriptionManagerRepositories(subRepo, subscriberRepo, topicRepo),
		WithSubscriptionManagerLogger(&NoopLogger{}),
	)
	require.NoError(t, err)
	return m
}

// --- Constructor tests ---

// TestNewSubscriptionManager_ValidOptions verifies a SubscriptionManager is created correctly.
func TestNewSubscriptionManager_ValidOptions(t *testing.T) {
	sm, err := NewSubscriptionManager(
		WithSubscriptionManagerRepositories(
			newMockSubscriptionRepository(),
			newMockSubscriberRepository(),
			newMockTopicRepositoryForManager(),
		),
		WithSubscriptionManagerLogger(&NoopLogger{}),
	)
	require.NoError(t, err)
	assert.NotNil(t, sm)
}

// TestNewSubscriptionManager_MissingSubscriptionRepo verifies that missing SubscriptionRepository
// returns an error.
func TestNewSubscriptionManager_MissingSubscriptionRepo(t *testing.T) {
	_, err := NewSubscriptionManager(
		WithSubscriptionManagerLogger(&NoopLogger{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SubscriptionRepository is required")
}

// TestNewSubscriptionManager_MissingLogger verifies that missing Logger returns an error.
func TestNewSubscriptionManager_MissingLogger(t *testing.T) {
	_, err := NewSubscriptionManager(
		WithSubscriptionManagerRepositories(
			newMockSubscriptionRepository(),
			newMockSubscriberRepository(),
			newMockTopicRepositoryForManager(),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Logger is required")
}

// TestNewSubscriptionManager_NilRepoInOption verifies that nil repositories return an error.
func TestNewSubscriptionManager_NilRepoInOption(t *testing.T) {
	_, err := NewSubscriptionManager(
		WithSubscriptionManagerRepositories(nil, nil, nil),
		WithSubscriptionManagerLogger(&NoopLogger{}),
	)
	require.Error(t, err)
}

// TestNewSubscriptionManager_NilLogger verifies that nil logger returns an error.
func TestNewSubscriptionManager_NilLogger(t *testing.T) {
	_, err := NewSubscriptionManager(
		WithSubscriptionManagerRepositories(
			newMockSubscriptionRepository(),
			newMockSubscriberRepository(),
			newMockTopicRepositoryForManager(),
		),
		WithSubscriptionManagerLogger(nil),
	)
	require.Error(t, err)
}

// --- Subscribe tests ---

// TestSubscribe_MissingSubscriberID verifies that a zero subscriber ID returns a validation error.
func TestSubscribe_MissingSubscriberID(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 0,
		TopicCode:    "user.signup",
		Identifier:   "user-1",
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "subscriber ID is required")
}

// TestSubscribe_MissingTopicCode verifies that an empty topic code returns a validation error.
func TestSubscribe_MissingTopicCode(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "",
		Identifier:   "user-1",
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "topic code is required")
}

// TestSubscribe_MissingIdentifier verifies that an empty identifier returns a validation error.
func TestSubscribe_MissingIdentifier(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "user.signup",
		Identifier:   "",
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "identifier is required")
}

// TestSubscribe_SubscriberNotFound verifies that a nonexistent subscriber returns a validation error.
func TestSubscribe_SubscriberNotFound(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(), // empty subscriber repo
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 999,
		TopicCode:    "user.signup",
		Identifier:   "user-1",
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "subscriber not found")
}

// TestSubscribe_SubscriberRepoError verifies that a subscriber repo error propagates correctly.
func TestSubscribe_SubscriberRepoError(t *testing.T) {
	subscriberRepo := newMockSubscriberRepository()
	subscriberRepo.loadErr = errors.New("subscriber database down")

	sm := buildManager(t,
		newMockSubscriptionRepository(),
		subscriberRepo,
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "user.signup",
		Identifier:   "user-1",
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
	assert.Contains(t, pubErr.Message, "failed to load subscriber")
}

// TestSubscribe_TopicNotFound verifies that a nonexistent topic returns a validation error.
func TestSubscribe_TopicNotFound(t *testing.T) {
	subscriberRepo := newMockSubscriberRepository()
	subscriberRepo.subscribers[1] = model.Subscriber{ID: 1, Name: "svc-a", IsActive: true}

	sm := buildManager(t,
		newMockSubscriptionRepository(),
		subscriberRepo,
		newMockTopicRepositoryForManager(), // empty topic repo
	)

	_, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "missing.topic",
		Identifier:   "user-1",
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "topic not found")
}

// TestSubscribe_TopicRepoError verifies that a topic repo database error propagates correctly.
func TestSubscribe_TopicRepoError(t *testing.T) {
	subscriberRepo := newMockSubscriberRepository()
	subscriberRepo.subscribers[1] = model.Subscriber{ID: 1, Name: "svc-a", IsActive: true}

	topicRepo := newMockTopicRepositoryForManager()
	topicRepo.getByCodeErr = errors.New("topic table locked")

	sm := buildManager(t, newMockSubscriptionRepository(), subscriberRepo, topicRepo)

	_, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "user.signup",
		Identifier:   "user-1",
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
	assert.Contains(t, pubErr.Message, "failed to load topic")
}

// TestSubscribe_CreatesNewSubscription verifies that a valid request creates a new subscription.
func TestSubscribe_CreatesNewSubscription(t *testing.T) {
	subscriberRepo := newMockSubscriberRepository()
	subscriberRepo.subscribers[1] = model.Subscriber{ID: 1, Name: "svc-a", IsActive: true}

	topicRepo := newMockTopicRepositoryForManager()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	subRepo := newMockSubscriptionRepository()

	sm := buildManager(t, subRepo, subscriberRepo, topicRepo)

	result, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "user.signup",
		Identifier:   "user-123",
		CallbackURL:  "https://svc-a.example.com/webhook",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.ID, int64(0))
	assert.Equal(t, int64(1), result.SubscriberID)
	assert.Equal(t, int64(10), result.TopicID)
	assert.Equal(t, "user-123", result.Identifier)
	assert.True(t, result.IsActive)

	// Verify it is persisted in the repo
	assert.Len(t, subRepo.subscriptions, 1)
}

// TestSubscribe_ReturnsExistingActiveSubscription verifies that subscribing twice with the same
// parameters returns the existing active subscription without creating a duplicate.
func TestSubscribe_ReturnsExistingActiveSubscription(t *testing.T) {
	subscriberRepo := newMockSubscriberRepository()
	subscriberRepo.subscribers[1] = model.Subscriber{ID: 1, Name: "svc-a", IsActive: true}

	topicRepo := newMockTopicRepositoryForManager()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	subRepo := newMockSubscriptionRepository()
	// Pre-existing active subscription
	existing := model.Subscription{ID: 5, SubscriberID: 1, TopicID: 10, Identifier: "user-123", IsActive: true}
	subRepo.activeItems = []model.Subscription{existing}

	sm := buildManager(t, subRepo, subscriberRepo, topicRepo)

	result, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "user.signup",
		Identifier:   "user-123",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	// Must return the existing subscription, not create a new one
	assert.Equal(t, int64(5), result.ID)
	// Repo must still have only the pre-seeded subscription (no new saves)
	assert.Len(t, subRepo.subscriptions, 0)
}

// TestSubscribe_SaveError verifies that a save error propagates correctly.
func TestSubscribe_SaveError(t *testing.T) {
	subscriberRepo := newMockSubscriberRepository()
	subscriberRepo.subscribers[1] = model.Subscriber{ID: 1, Name: "svc-a", IsActive: true}

	topicRepo := newMockTopicRepositoryForManager()
	topicRepo.topics["user.signup"] = model.Topic{ID: 10, Code: "user.signup", Name: "User Signup", IsActive: true}

	subRepo := newMockSubscriptionRepository()
	subRepo.saveErr = errors.New("disk full")

	sm := buildManager(t, subRepo, subscriberRepo, topicRepo)

	_, err := sm.Subscribe(context.Background(), SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "user.signup",
		Identifier:   "user-123",
	})

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
	assert.Contains(t, pubErr.Message, "failed to save subscription")
}

// --- Unsubscribe tests ---

// TestUnsubscribe_ZeroID verifies that a zero subscription ID returns a validation error.
func TestUnsubscribe_ZeroID(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.Unsubscribe(context.Background(), 0)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "subscription ID is required")
}

// TestUnsubscribe_NotFound verifies that unsubscribing a nonexistent ID returns a validation error.
func TestUnsubscribe_NotFound(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(), // empty
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.Unsubscribe(context.Background(), 999)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "subscription not found")
}

// TestUnsubscribe_LoadError verifies that a load error propagates correctly.
func TestUnsubscribe_LoadError(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	subRepo.loadErr = errors.New("subscription table down")

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	_, err := sm.Unsubscribe(context.Background(), 1)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
}

// TestUnsubscribe_ActiveSubscription verifies that an active subscription is deactivated.
func TestUnsubscribe_ActiveSubscription(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	subRepo.subscriptions[1] = model.Subscription{ID: 1, SubscriberID: 1, TopicID: 10, Identifier: "user-1", IsActive: true}

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	result, err := sm.Unsubscribe(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsActive)
	assert.True(t, result.DeletedAt.Valid)

	// Verify it was saved as inactive
	saved, ok := subRepo.subscriptions[1]
	require.True(t, ok)
	assert.False(t, saved.IsActive)
}

// TestUnsubscribe_AlreadyInactive verifies that unsubscribing an already-inactive subscription
// returns without error and without re-saving.
func TestUnsubscribe_AlreadyInactive(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	inactiveSub := model.Subscription{ID: 1, SubscriberID: 1, TopicID: 10, Identifier: "user-1", IsActive: false}
	subRepo.subscriptions[1] = inactiveSub

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	result, err := sm.Unsubscribe(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsActive)
}

// TestUnsubscribe_SaveError verifies that a save error during deactivation propagates correctly.
func TestUnsubscribe_SaveError(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	subRepo.subscriptions[1] = model.Subscription{ID: 1, SubscriberID: 1, TopicID: 10, Identifier: "user-1", IsActive: true}
	subRepo.saveErr = errors.New("write failed")

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	_, err := sm.Unsubscribe(context.Background(), 1)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
	assert.Contains(t, pubErr.Message, "failed to save subscription")
}

// --- ListSubscriptions tests ---

// TestListSubscriptions_ZeroSubscriberID verifies that a zero subscriber ID returns a validation error.
func TestListSubscriptions_ZeroSubscriberID(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.ListSubscriptions(context.Background(), 0, "")

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "subscriber ID is required")
}

// TestListSubscriptions_EmptyReturnsNil verifies that no subscriptions returns an empty (not nil) slice.
func TestListSubscriptions_EmptyReturnsNil(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	result, err := sm.ListSubscriptions(context.Background(), 1, "")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// TestListSubscriptions_ReturnsMatchingSubscriptions verifies that active subscriptions are returned.
func TestListSubscriptions_ReturnsMatchingSubscriptions(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	subRepo.activeItems = []model.Subscription{
		{ID: 1, SubscriberID: 1, TopicID: 10, Identifier: "user-123", IsActive: true},
		{ID: 2, SubscriberID: 1, TopicID: 11, Identifier: "order-456", IsActive: true},
	}

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	result, err := sm.ListSubscriptions(context.Background(), 1, "")

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

// TestListSubscriptions_RepoError verifies that a repository error is returned as a database error.
func TestListSubscriptions_RepoError(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	subRepo.findActiveErr = errors.New("subscription query timeout")

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	_, err := sm.ListSubscriptions(context.Background(), 1, "")

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
}

// --- GetSubscription tests ---

// TestGetSubscription_ZeroID verifies that a zero ID returns a validation error.
func TestGetSubscription_ZeroID(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.GetSubscription(context.Background(), 0)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
}

// TestGetSubscription_Found verifies that a valid ID returns the correct subscription.
func TestGetSubscription_Found(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	subRepo.subscriptions[7] = model.Subscription{ID: 7, SubscriberID: 1, TopicID: 10, Identifier: "user-1", IsActive: true}

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	result, err := sm.GetSubscription(context.Background(), 7)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(7), result.ID)
}

// TestGetSubscription_NotFound verifies that a missing subscription returns a validation error.
func TestGetSubscription_NotFound(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.GetSubscription(context.Background(), 999)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
	assert.Contains(t, pubErr.Message, "subscription not found")
}

// --- ReactivateSubscription tests ---

// TestReactivateSubscription_ZeroID verifies that a zero ID returns a validation error.
func TestReactivateSubscription_ZeroID(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.ReactivateSubscription(context.Background(), 0)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
}

// TestReactivateSubscription_NotFound verifies that a missing subscription returns a validation error.
func TestReactivateSubscription_NotFound(t *testing.T) {
	sm := buildManager(t,
		newMockSubscriptionRepository(),
		newMockSubscriberRepository(),
		newMockTopicRepositoryForManager(),
	)

	_, err := sm.ReactivateSubscription(context.Background(), 999)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeValidation, pubErr.Code)
}

// TestReactivateSubscription_ReactivatesInactive verifies that an inactive subscription is reactivated.
func TestReactivateSubscription_ReactivatesInactive(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	sub := model.Subscription{ID: 3, SubscriberID: 1, TopicID: 10, Identifier: "user-1", IsActive: false}
	subRepo.subscriptions[3] = sub

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	result, err := sm.ReactivateSubscription(context.Background(), 3)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsActive)
	assert.False(t, result.DeletedAt.Valid)

	// Verify persisted as active
	saved, ok := subRepo.subscriptions[3]
	require.True(t, ok)
	assert.True(t, saved.IsActive)
}

// TestReactivateSubscription_AlreadyActive verifies that reactivating an already-active subscription
// returns without error and without re-saving.
func TestReactivateSubscription_AlreadyActive(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	subRepo.subscriptions[4] = model.Subscription{ID: 4, SubscriberID: 1, TopicID: 10, Identifier: "user-1", IsActive: true}

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	result, err := sm.ReactivateSubscription(context.Background(), 4)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsActive)
}

// TestReactivateSubscription_SaveError verifies that a save error propagates correctly.
func TestReactivateSubscription_SaveError(t *testing.T) {
	subRepo := newMockSubscriptionRepository()
	sub := model.Subscription{ID: 5, SubscriberID: 1, TopicID: 10, Identifier: "user-1", IsActive: false}
	subRepo.subscriptions[5] = sub
	subRepo.saveErr = errors.New("write failed")

	sm := buildManager(t, subRepo, newMockSubscriberRepository(), newMockTopicRepositoryForManager())

	_, err := sm.ReactivateSubscription(context.Background(), 5)

	require.Error(t, err)
	var pubErr *Error
	require.True(t, errors.As(err, &pubErr))
	assert.Equal(t, ErrCodeDatabase, pubErr.Code)
}
