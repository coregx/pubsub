// Package api provides HTTP handler tests for the PubSub server REST API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coregx/fursy"
	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock repository implementations ---

// mockMessageRepo implements pubsub.MessageRepository for handler tests.
type mockMessageRepo struct {
	nextID  int64
	saveErr error
}

func newMockMessageRepo() *mockMessageRepo {
	return &mockMessageRepo{nextID: 1}
}

func (m *mockMessageRepo) Load(_ context.Context, _ int64) (model.Message, error) {
	return model.Message{}, pubsub.ErrNoData
}

func (m *mockMessageRepo) Save(_ context.Context, msg model.Message) (model.Message, error) {
	if m.saveErr != nil {
		return model.Message{}, m.saveErr
	}
	if msg.ID == 0 {
		msg.ID = m.nextID
		m.nextID++
	}
	return msg, nil
}

func (m *mockMessageRepo) Delete(_ context.Context, _ model.Message) error {
	return nil
}

func (m *mockMessageRepo) FindOutdatedMessages(_ context.Context, _ int) ([]model.Message, error) {
	return nil, pubsub.ErrNoData
}

// mockQueueRepo implements pubsub.QueueRepository for handler tests.
type mockQueueRepo struct {
	items  map[int64]*model.Queue
	nextID int64
}

func newMockQueueRepo() *mockQueueRepo {
	return &mockQueueRepo{
		items:  make(map[int64]*model.Queue),
		nextID: 1,
	}
}

func (m *mockQueueRepo) Load(_ context.Context, id int64) (model.Queue, error) {
	if item, ok := m.items[id]; ok {
		return *item, nil
	}
	return model.Queue{}, pubsub.ErrNoData
}

func (m *mockQueueRepo) Save(_ context.Context, q *model.Queue) (*model.Queue, error) {
	if q.ID == 0 {
		q.ID = m.nextID
		m.nextID++
	}
	saved := *q
	m.items[q.ID] = &saved
	return &saved, nil
}

func (m *mockQueueRepo) Delete(_ context.Context, _ *model.Queue) error {
	return nil
}

func (m *mockQueueRepo) FindByMessageID(_ context.Context, _, _ int64) (model.Queue, error) {
	return model.Queue{}, pubsub.ErrNoData
}

func (m *mockQueueRepo) FindBySubscriptionID(_ context.Context, _ int64) ([]model.Queue, error) {
	return nil, pubsub.ErrNoData
}

func (m *mockQueueRepo) FindPendingItems(_ context.Context, _ int) ([]model.Queue, error) {
	return nil, pubsub.ErrNoData
}

func (m *mockQueueRepo) FindRetryableItems(_ context.Context, _ int) ([]model.Queue, error) {
	return nil, pubsub.ErrNoData
}

func (m *mockQueueRepo) FindExpiredItems(_ context.Context, _ int) ([]model.Queue, error) {
	return nil, pubsub.ErrNoData
}

func (m *mockQueueRepo) UpdateNextRetry(_ context.Context, _ int64, _ time.Time, _ int) error {
	return nil
}

// mockSubscriptionRepo implements pubsub.SubscriptionRepository for handler tests.
type mockSubscriptionRepo struct {
	subscriptions map[int64]model.Subscription
	activeItems   []model.Subscription
	loadErr       error
	saveErr       error
	findActiveErr error
	nextID        int64
}

func newMockSubscriptionRepo() *mockSubscriptionRepo {
	return &mockSubscriptionRepo{
		subscriptions: make(map[int64]model.Subscription),
		nextID:        1,
	}
}

func (m *mockSubscriptionRepo) Load(_ context.Context, id int64) (model.Subscription, error) {
	if m.loadErr != nil {
		return model.Subscription{}, m.loadErr
	}
	if sub, ok := m.subscriptions[id]; ok {
		return sub, nil
	}
	return model.Subscription{}, pubsub.ErrNoData
}

func (m *mockSubscriptionRepo) Save(_ context.Context, sub model.Subscription) (model.Subscription, error) {
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

func (m *mockSubscriptionRepo) FindActive(_ context.Context, _ int64, _ string) ([]model.Subscription, error) {
	if m.findActiveErr != nil {
		return nil, m.findActiveErr
	}
	if m.activeItems != nil {
		return m.activeItems, nil
	}
	return nil, pubsub.ErrNoData
}

func (m *mockSubscriptionRepo) List(_ context.Context, _ pubsub.Filter) ([]model.Subscription, error) {
	result := make([]model.Subscription, 0, len(m.subscriptions))
	for _, sub := range m.subscriptions {
		result = append(result, sub)
	}
	return result, nil
}

func (m *mockSubscriptionRepo) FindAllActive(_ context.Context) ([]model.SubscriptionFull, error) {
	return nil, pubsub.ErrNoData
}

// mockTopicRepo implements pubsub.TopicRepository for handler tests.
type mockTopicRepo struct {
	topics       map[string]model.Topic
	getByCodeErr error
}

func newMockTopicRepo() *mockTopicRepo {
	return &mockTopicRepo{
		topics: make(map[string]model.Topic),
	}
}

func (m *mockTopicRepo) Load(_ context.Context, id int64) (model.Topic, error) {
	for _, t := range m.topics {
		if t.ID == id {
			return t, nil
		}
	}
	return model.Topic{}, pubsub.ErrNoData
}

func (m *mockTopicRepo) Save(_ context.Context, t model.Topic) (model.Topic, error) {
	m.topics[t.Code] = t
	return t, nil
}

func (m *mockTopicRepo) GetByTopicCode(_ context.Context, code string) (model.Topic, error) {
	if m.getByCodeErr != nil {
		return model.Topic{}, m.getByCodeErr
	}
	if t, ok := m.topics[code]; ok {
		return t, nil
	}
	return model.Topic{}, pubsub.ErrNoData
}

// mockSubscriberRepo implements pubsub.SubscriberRepository for handler tests.
type mockSubscriberRepo struct {
	subscribers map[int64]model.Subscriber
	loadErr     error
}

func newMockSubscriberRepo() *mockSubscriberRepo {
	return &mockSubscriberRepo{
		subscribers: make(map[int64]model.Subscriber),
	}
}

func (m *mockSubscriberRepo) Load(_ context.Context, id int64) (model.Subscriber, error) {
	if m.loadErr != nil {
		return model.Subscriber{}, m.loadErr
	}
	if sub, ok := m.subscribers[id]; ok {
		return sub, nil
	}
	return model.Subscriber{}, pubsub.ErrNoData
}

func (m *mockSubscriberRepo) Save(_ context.Context, sub model.Subscriber) (model.Subscriber, error) {
	m.subscribers[sub.ID] = sub
	return sub, nil
}

func (m *mockSubscriberRepo) FindByClientID(_ context.Context, _ int64) (model.Subscriber, error) {
	return model.Subscriber{}, pubsub.ErrNoData
}

func (m *mockSubscriberRepo) FindByName(_ context.Context, _ string) (model.Subscriber, error) {
	return model.Subscriber{}, pubsub.ErrNoData
}

// --- Test fixtures ---

// testDeps holds the mock repositories and constructed services for a test.
type testDeps struct {
	msgRepo          *mockMessageRepo
	queueRepo        *mockQueueRepo
	subscriptionRepo *mockSubscriptionRepo
	topicRepo        *mockTopicRepo
	subscriberRepo   *mockSubscriberRepo
	publisher        *pubsub.Publisher
	subManager       *pubsub.SubscriptionManager
}

// newTestDeps creates mock repos and constructs Publisher + SubscriptionManager.
func newTestDeps(t *testing.T) *testDeps {
	t.Helper()

	msgRepo := newMockMessageRepo()
	queueRepo := newMockQueueRepo()
	subRepo := newMockSubscriptionRepo()
	topicRepo := newMockTopicRepo()
	subscriberRepo := newMockSubscriberRepo()

	publisher, err := pubsub.NewPublisher(
		pubsub.WithPublisherRepositories(msgRepo, queueRepo, subRepo, topicRepo),
		pubsub.WithPublisherLogger(&pubsub.NoopLogger{}),
	)
	require.NoError(t, err)

	subManager, err := pubsub.NewSubscriptionManager(
		pubsub.WithSubscriptionManagerRepositories(subRepo, subscriberRepo, topicRepo),
		pubsub.WithSubscriptionManagerLogger(&pubsub.NoopLogger{}),
	)
	require.NoError(t, err)

	return &testDeps{
		msgRepo:          msgRepo,
		queueRepo:        queueRepo,
		subscriptionRepo: subRepo,
		topicRepo:        topicRepo,
		subscriberRepo:   subscriberRepo,
		publisher:        publisher,
		subManager:       subManager,
	}
}

// setupTestRouter creates a Fursy router with all routes registered.
func setupTestRouter(t *testing.T, deps *testDeps) *fursy.Router {
	t.Helper()
	router := fursy.New()
	handler := NewHandler(deps.publisher, deps.subManager, &pubsub.NoopLogger{})
	handler.RegisterRoutes(router)
	return router
}

// jsonBody serializes v to a *bytes.Buffer for use as a request body.
func jsonBody(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

// --- Health endpoint tests ---

// TestHandleHealth_OK verifies that GET /api/v1/health returns 200 with correct fields.
func TestHandleHealth_OK(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp HealthResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp.Status)
	assert.Equal(t, "0.2.0", resp.Version)
	assert.False(t, resp.Timestamp.IsZero())
}

// TestHandleHealth_TimestampIsUTC verifies the health response timestamp is in UTC.
func TestHandleHealth_TimestampIsUTC(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	before := time.Now().UTC()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	after := time.Now().UTC()

	var resp HealthResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.False(t, resp.Timestamp.Before(before), "timestamp should not be before test start")
	assert.False(t, resp.Timestamp.After(after), "timestamp should not be after test end")
}

// --- Publish endpoint tests ---

// TestHandlePublish_Created verifies that a valid publish request returns 201 Created.
func TestHandlePublish_Created(t *testing.T) {
	deps := newTestDeps(t)
	// Register a topic so Publish can resolve it.
	deps.topicRepo.topics["user.signup"] = model.Topic{
		ID: 1, Code: "user.signup", Name: "User Signup", IsActive: true,
	}
	router := setupTestRouter(t, deps)

	body := jsonBody(t, PublishRequest{
		TopicCode:  "user.signup",
		Identifier: "user-123",
		Data:       json.RawMessage(`{"name":"Alice"}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp PublishResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "Message published successfully", resp.Message)
}

// TestHandlePublish_MissingTopicCode verifies that publishing without a topic returns an error.
// The handler calls publisher.Publish() which validates the topic code and returns a validation
// error; the handler then returns 500 (InternalServerError problem).
func TestHandlePublish_MissingTopicCode(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	// Send empty topicCode — publisher.Publish() will return validation error,
	// handler maps it to InternalServerError (no topic-specific 4xx in handler).
	body := jsonBody(t, PublishRequest{
		TopicCode:  "",
		Identifier: "user-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Handler maps all Publish errors to 500 InternalServerError Problem.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHandlePublish_UnknownTopic verifies that publishing to a nonexistent topic returns 500.
func TestHandlePublish_UnknownTopic(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	body := jsonBody(t, PublishRequest{
		TopicCode:  "nonexistent.topic",
		Identifier: "event-1",
		Data:       json.RawMessage(`{}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHandlePublish_NoActiveSubscriptions verifies publish succeeds with 0 queue items.
func TestHandlePublish_NoActiveSubscriptions(t *testing.T) {
	deps := newTestDeps(t)
	deps.topicRepo.topics["alerts"] = model.Topic{
		ID: 2, Code: "alerts", Name: "Alerts", IsActive: true,
	}
	router := setupTestRouter(t, deps)

	body := jsonBody(t, PublishRequest{
		TopicCode:  "alerts",
		Identifier: "alert-001",
		Data:       json.RawMessage(`{"severity":"low"}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp PublishResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

// --- Subscribe endpoint tests ---

// TestHandleSubscribe_Created verifies that a valid subscribe request returns 201 Created.
func TestHandleSubscribe_Created(t *testing.T) {
	deps := newTestDeps(t)
	deps.topicRepo.topics["user.signup"] = model.Topic{
		ID: 1, Code: "user.signup", Name: "User Signup", IsActive: true,
	}
	deps.subscriberRepo.subscribers[42] = model.Subscriber{
		ID: 42, Name: "svc-notifications", IsActive: true,
	}
	router := setupTestRouter(t, deps)

	body := jsonBody(t, SubscribeRequest{
		SubscriberID: 42,
		TopicCode:    "user.signup",
		Identifier:   "user-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscribe", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp SubscriptionResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "Subscription created successfully", resp.Message)
	require.NotNil(t, resp.Data)
	assert.Greater(t, resp.Data.ID, int64(0))
}

// TestHandleSubscribe_MissingSubscriberID verifies that subscribing without subscriber returns 500.
// SubscriptionManager.Subscribe validates SubscriberID > 0; handler maps errors to 500.
func TestHandleSubscribe_MissingSubscriberID(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	body := jsonBody(t, SubscribeRequest{
		SubscriberID: 0,
		TopicCode:    "user.signup",
		Identifier:   "user-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscribe", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHandleSubscribe_UnknownSubscriber verifies that subscribing with a nonexistent subscriber ID returns 500.
func TestHandleSubscribe_UnknownSubscriber(t *testing.T) {
	deps := newTestDeps(t)
	deps.topicRepo.topics["user.signup"] = model.Topic{
		ID: 1, Code: "user.signup", Name: "User Signup", IsActive: true,
	}
	router := setupTestRouter(t, deps)

	body := jsonBody(t, SubscribeRequest{
		SubscriberID: 999,
		TopicCode:    "user.signup",
		Identifier:   "user-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscribe", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHandleSubscribe_DuplicateReturnsExisting verifies that a duplicate subscription returns the existing one.
func TestHandleSubscribe_DuplicateReturnsExisting(t *testing.T) {
	deps := newTestDeps(t)
	deps.topicRepo.topics["user.signup"] = model.Topic{
		ID: 1, Code: "user.signup", Name: "User Signup", IsActive: true,
	}
	deps.subscriberRepo.subscribers[10] = model.Subscriber{
		ID: 10, Name: "svc-a", IsActive: true,
	}
	// Pre-seed an active subscription so Subscribe returns the existing one.
	existing := model.Subscription{
		ID: 5, SubscriberID: 10, TopicID: 1, Identifier: "user-123", IsActive: true,
	}
	deps.subscriptionRepo.activeItems = []model.Subscription{existing}
	router := setupTestRouter(t, deps)

	body := jsonBody(t, SubscribeRequest{
		SubscriberID: 10,
		TopicCode:    "user.signup",
		Identifier:   "user-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscribe", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp SubscriptionResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	require.NotNil(t, resp.Data)
	// Must return the pre-existing subscription, not a new one.
	assert.Equal(t, int64(5), resp.Data.ID)
}

// --- List subscriptions endpoint tests ---

// TestHandleListSubscriptions_EmptyList verifies 200 with empty data when no subscriptions exist.
func TestHandleListSubscriptions_EmptyList(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	// No subscriberID → ListSubscriptions gets 0, which returns validation error →
	// handler catches ErrNoData branch (subscriberID=0 is a validation error, not no-data).
	// Test with valid subscriberID to hit the empty-list branch.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions?subscriberID=1", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// subscriberRepo has no subscriptions, FindActive returns ErrNoData →
	// ListSubscriptions returns empty slice → handler returns 200 with empty data.
	assert.Equal(t, http.StatusOK, w.Code)

	var resp SubscriptionListResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
	assert.Empty(t, resp.Data)
}

// TestHandleListSubscriptions_WithResults verifies 200 with populated data.
func TestHandleListSubscriptions_WithResults(t *testing.T) {
	deps := newTestDeps(t)
	deps.subscriptionRepo.activeItems = []model.Subscription{
		{ID: 1, SubscriberID: 7, TopicID: 1, Identifier: "event-a", IsActive: true},
		{ID: 2, SubscriberID: 7, TopicID: 2, Identifier: "event-b", IsActive: true},
	}
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions?subscriberID=7", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SubscriptionListResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 2)
}

// TestHandleListSubscriptions_NoSubscriberID verifies behavior with missing subscriberID param.
// Without subscriberID, strconv.ParseInt returns 0, ListSubscriptions returns validation error.
func TestHandleListSubscriptions_NoSubscriberID(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// subscriberID=0 → ListSubscriptions returns validation error (not ErrNoData) →
	// handler hits the non-ErrNoData branch → returns 500 InternalServerError Problem.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHandleListSubscriptions_WithIdentifierFilter verifies query param filtering is accepted.
func TestHandleListSubscriptions_WithIdentifierFilter(t *testing.T) {
	deps := newTestDeps(t)
	deps.subscriptionRepo.activeItems = []model.Subscription{
		{ID: 3, SubscriberID: 5, TopicID: 10, Identifier: "order-42", IsActive: true},
	}
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/subscriptions?subscriberID=5&identifier=order-42",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SubscriptionListResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, int64(3), resp.Data[0].ID)
}

// --- Unsubscribe endpoint tests ---

// TestHandleUnsubscribe_OK verifies that deleting an existing subscription returns 200.
func TestHandleUnsubscribe_OK(t *testing.T) {
	deps := newTestDeps(t)
	deps.subscriptionRepo.subscriptions[1] = model.Subscription{
		ID: 1, SubscriberID: 10, TopicID: 1, Identifier: "user-1", IsActive: true,
	}
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/1", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp PublishResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "Unsubscribed successfully", resp.Message)
}

// TestHandleUnsubscribe_NotFound verifies that deleting a nonexistent subscription returns 500.
// SubscriptionManager.Unsubscribe wraps ErrNoData in ErrCodeValidation, so pubsub.IsNoData
// returns false for the wrapped error. The handler therefore falls through to InternalServerError.
func TestHandleUnsubscribe_NotFound(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/999", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// ErrCodeValidation wraps ErrNoData — IsNoData returns false — handler returns 500.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHandleUnsubscribe_InvalidID verifies that a non-numeric ID returns 400 Bad Request.
func TestHandleUnsubscribe_InvalidID(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/abc", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandleUnsubscribe_AlreadyInactive verifies that deleting an already-inactive subscription returns 200.
func TestHandleUnsubscribe_AlreadyInactive(t *testing.T) {
	deps := newTestDeps(t)
	deps.subscriptionRepo.subscriptions[2] = model.Subscription{
		ID: 2, SubscriberID: 10, TopicID: 1, Identifier: "user-2", IsActive: false,
	}
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/2", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Unsubscribe on already-inactive subscription returns the subscription without error.
	assert.Equal(t, http.StatusOK, w.Code)

	var resp PublishResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

// --- Response content-type tests ---

// TestHandleHealth_ContentType verifies that responses carry application/json content type.
func TestHandleHealth_ContentType(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// TestHandleUnsubscribe_ProblemContentType verifies that error responses use problem+json.
func TestHandleUnsubscribe_ProblemContentType(t *testing.T) {
	deps := newTestDeps(t)
	router := setupTestRouter(t, deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/abc", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
}

// --- Location header tests ---

// TestHandlePublish_LocationHeader verifies that 201 responses include a Location header.
func TestHandlePublish_LocationHeader(t *testing.T) {
	deps := newTestDeps(t)
	deps.topicRepo.topics["orders"] = model.Topic{
		ID: 3, Code: "orders", Name: "Orders", IsActive: true,
	}
	router := setupTestRouter(t, deps)

	body := jsonBody(t, PublishRequest{
		TopicCode:  "orders",
		Identifier: "order-999",
		Data:       json.RawMessage(`{}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "/api/v1/messages", w.Header().Get("Location"))
}

// TestHandleSubscribe_LocationHeader verifies that 201 subscribe responses include a Location header.
func TestHandleSubscribe_LocationHeader(t *testing.T) {
	deps := newTestDeps(t)
	deps.topicRepo.topics["user.signup"] = model.Topic{
		ID: 1, Code: "user.signup", Name: "User Signup", IsActive: true,
	}
	deps.subscriberRepo.subscribers[1] = model.Subscriber{
		ID: 1, Name: "svc-a", IsActive: true,
	}
	router := setupTestRouter(t, deps)

	body := jsonBody(t, SubscribeRequest{
		SubscriberID: 1,
		TopicCode:    "user.signup",
		Identifier:   "user-42",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscribe", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "/api/v1/subscriptions", w.Header().Get("Location"))
}
