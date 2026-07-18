package pubsub

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/coregx/pubsub/model"
	"github.com/coregx/pubsub/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Queue repository mock ---

// mockQueueRepository implements QueueRepository for QueueWorker tests.
type mockQueueRepository struct {
	items        map[int64]*model.Queue
	nextID       int64
	pendingItems []model.Queue
	retryItems   []model.Queue
	expiredItems []model.Queue
	saveErr      error
	deleteErr    error
	pendingErr   error
	retryErr     error
	expiredErr   error
	savedItems   []*model.Queue
	deletedItems []*model.Queue
}

func newMockQueueRepository() *mockQueueRepository {
	return &mockQueueRepository{
		items:  make(map[int64]*model.Queue),
		nextID: 1,
	}
}

func (m *mockQueueRepository) Load(_ context.Context, id int64) (model.Queue, error) {
	if item, ok := m.items[id]; ok {
		return *item, nil
	}
	return model.Queue{}, ErrNoData
}

func (m *mockQueueRepository) Save(_ context.Context, q *model.Queue) (*model.Queue, error) {
	if m.saveErr != nil {
		return nil, m.saveErr
	}
	if q.ID == 0 {
		q.ID = m.nextID
		m.nextID++
	}
	cp := *q
	m.items[q.ID] = &cp
	m.savedItems = append(m.savedItems, &cp)
	return &cp, nil
}

func (m *mockQueueRepository) Delete(_ context.Context, q *model.Queue) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.items, q.ID)
	cp := *q
	m.deletedItems = append(m.deletedItems, &cp)
	return nil
}

func (m *mockQueueRepository) FindByMessageID(_ context.Context, _, _ int64) (model.Queue, error) {
	return model.Queue{}, ErrNoData
}

func (m *mockQueueRepository) FindBySubscriptionID(_ context.Context, _ int64) ([]model.Queue, error) {
	return nil, ErrNoData
}

func (m *mockQueueRepository) FindPendingItems(_ context.Context, _ int) ([]model.Queue, error) {
	if m.pendingErr != nil {
		return nil, m.pendingErr
	}
	if len(m.pendingItems) == 0 {
		return nil, ErrNoData
	}
	return m.pendingItems, nil
}

func (m *mockQueueRepository) FindRetryableItems(_ context.Context, _ int) ([]model.Queue, error) {
	if m.retryErr != nil {
		return nil, m.retryErr
	}
	if len(m.retryItems) == 0 {
		return nil, ErrNoData
	}
	return m.retryItems, nil
}

func (m *mockQueueRepository) FindExpiredItems(_ context.Context, _ int) ([]model.Queue, error) {
	if m.expiredErr != nil {
		return nil, m.expiredErr
	}
	if len(m.expiredItems) == 0 {
		return nil, ErrNoData
	}
	return m.expiredItems, nil
}

func (m *mockQueueRepository) UpdateNextRetry(_ context.Context, _ int64, _ time.Time, _ int) error {
	return nil
}

// --- Message repository mock ---

// mockMessageRepositoryForWorker implements MessageRepository for QueueWorker tests.
type mockMessageRepositoryForWorker struct {
	messages map[int64]model.Message
	loadErr  error
}

func newMockMessageRepositoryForWorker() *mockMessageRepositoryForWorker {
	return &mockMessageRepositoryForWorker{
		messages: make(map[int64]model.Message),
	}
}

func (m *mockMessageRepositoryForWorker) Load(_ context.Context, id int64) (model.Message, error) {
	if m.loadErr != nil {
		return model.Message{}, m.loadErr
	}
	if msg, ok := m.messages[id]; ok {
		return msg, nil
	}
	return model.Message{}, ErrNoData
}

func (m *mockMessageRepositoryForWorker) Save(_ context.Context, msg model.Message) (model.Message, error) {
	m.messages[msg.ID] = msg
	return msg, nil
}

func (m *mockMessageRepositoryForWorker) Delete(_ context.Context, _ model.Message) error {
	return nil
}

func (m *mockMessageRepositoryForWorker) FindOutdatedMessages(_ context.Context, _ int) ([]model.Message, error) {
	return nil, ErrNoData
}

// --- Subscription repository mock ---

// mockSubscriptionRepositoryForWorker implements SubscriptionRepository for QueueWorker tests.
type mockSubscriptionRepositoryForWorker struct {
	subscriptions map[int64]model.Subscription
	loadErr       error
}

func newMockSubscriptionRepositoryForWorker() *mockSubscriptionRepositoryForWorker {
	return &mockSubscriptionRepositoryForWorker{
		subscriptions: make(map[int64]model.Subscription),
	}
}

func (m *mockSubscriptionRepositoryForWorker) Load(_ context.Context, id int64) (model.Subscription, error) {
	if m.loadErr != nil {
		return model.Subscription{}, m.loadErr
	}
	if sub, ok := m.subscriptions[id]; ok {
		return sub, nil
	}
	return model.Subscription{}, ErrNoData
}

func (m *mockSubscriptionRepositoryForWorker) Save(_ context.Context, sub model.Subscription) (model.Subscription, error) {
	m.subscriptions[sub.ID] = sub
	return sub, nil
}

func (m *mockSubscriptionRepositoryForWorker) FindActive(_ context.Context, _ int64, _ string) ([]model.Subscription, error) {
	return nil, ErrNoData
}

func (m *mockSubscriptionRepositoryForWorker) List(_ context.Context, _ Filter) ([]model.Subscription, error) {
	return nil, ErrNoData
}

func (m *mockSubscriptionRepositoryForWorker) FindAllActive(_ context.Context) ([]model.SubscriptionFull, error) {
	return nil, ErrNoData
}

// --- DLQ repository mock ---

// mockDLQRepository implements DLQRepository for QueueWorker tests.
type mockDLQRepository struct {
	items    map[int64]model.DeadLetterQueue
	nextID   int64
	saveErr  error
	savedDLQ []model.DeadLetterQueue
}

func newMockDLQRepository() *mockDLQRepository {
	return &mockDLQRepository{
		items:  make(map[int64]model.DeadLetterQueue),
		nextID: 1,
	}
}

func (m *mockDLQRepository) Load(_ context.Context, id int64) (model.DeadLetterQueue, error) {
	if item, ok := m.items[id]; ok {
		return item, nil
	}
	return model.DeadLetterQueue{}, ErrNoData
}

func (m *mockDLQRepository) Save(_ context.Context, dlq model.DeadLetterQueue) (model.DeadLetterQueue, error) {
	if m.saveErr != nil {
		return model.DeadLetterQueue{}, m.saveErr
	}
	if dlq.ID == 0 {
		dlq.ID = m.nextID
		m.nextID++
	}
	m.items[dlq.ID] = dlq
	m.savedDLQ = append(m.savedDLQ, dlq)
	return dlq, nil
}

func (m *mockDLQRepository) Delete(_ context.Context, _ model.DeadLetterQueue) error {
	return nil
}

func (m *mockDLQRepository) FindBySubscription(_ context.Context, _ int64, _ int) ([]model.DeadLetterQueue, error) {
	return nil, ErrNoData
}

func (m *mockDLQRepository) FindUnresolved(_ context.Context, _ int) ([]model.DeadLetterQueue, error) {
	return nil, ErrNoData
}

func (m *mockDLQRepository) FindOlderThan(_ context.Context, _ time.Duration, _ int) ([]model.DeadLetterQueue, error) {
	return nil, ErrNoData
}

func (m *mockDLQRepository) FindByMessageID(_ context.Context, _ int64) (model.DeadLetterQueue, error) {
	return model.DeadLetterQueue{}, ErrNoData
}

func (m *mockDLQRepository) GetStats(_ context.Context) (model.DLQStats, error) {
	return model.DLQStats{TotalItems: len(m.items)}, nil
}

func (m *mockDLQRepository) CountUnresolved(_ context.Context) (int, error) {
	count := 0
	for id := range m.items {
		if !m.items[id].IsResolved {
			count++
		}
	}
	return count, nil
}

// --- Delivery mocks ---

// mockTransmitterProvider implements TransmitterProvider for testing.
type mockTransmitterProvider struct {
	callbackURL    string
	callbackURLErr error
}

//nolint:revive // GetCallbackUrl matches TransmitterProvider interface name defined in queue_worker.go
func (m *mockTransmitterProvider) GetCallbackUrl(_ context.Context, _ int64) (string, error) {
	if m.callbackURLErr != nil {
		return "", m.callbackURLErr
	}
	return m.callbackURL, nil
}

// mockDeliveryGateway implements MessageDeliveryGateway for testing.
type mockDeliveryGateway struct {
	deliverErr   error
	deliverCalls int
}

func (m *mockDeliveryGateway) DeliverMessage(_ context.Context, _ string, _ *model.DataMessage) error {
	m.deliverCalls++
	return m.deliverErr
}

// --- Worker builder helper ---

// buildWorker constructs a QueueWorker with provided dependencies.
func buildWorker(
	t *testing.T,
	qr QueueRepository,
	mr MessageRepository,
	sr SubscriptionRepository,
	dlqr DLQRepository,
	provider TransmitterProvider,
	gateway MessageDeliveryGateway,
) *QueueWorker {
	t.Helper()
	w, err := NewQueueWorker(
		WithRepositories(qr, mr, sr, dlqr),
		WithDelivery(provider, gateway),
		WithLogger(&NoopLogger{}),
	)
	require.NoError(t, err)
	return w
}

// --- Constructor tests ---

// TestNewQueueWorker_ValidOptions verifies a QueueWorker is created when all options are provided.
func TestNewQueueWorker_ValidOptions(t *testing.T) {
	w, err := NewQueueWorker(
		WithRepositories(
			newMockQueueRepository(),
			newMockMessageRepositoryForWorker(),
			newMockSubscriptionRepositoryForWorker(),
			newMockDLQRepository(),
		),
		WithDelivery(
			&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
			&mockDeliveryGateway{},
		),
		WithLogger(&NoopLogger{}),
	)
	require.NoError(t, err)
	assert.NotNil(t, w)
}

// TestNewQueueWorker_MissingQueueRepo verifies that missing QueueRepository returns an error.
func TestNewQueueWorker_MissingQueueRepo(t *testing.T) {
	_, err := NewQueueWorker(
		WithDelivery(
			&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
			&mockDeliveryGateway{},
		),
		WithLogger(&NoopLogger{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "QueueRepository is required")
}

// TestNewQueueWorker_MissingDelivery verifies that missing delivery dependency returns an error.
func TestNewQueueWorker_MissingDelivery(t *testing.T) {
	_, err := NewQueueWorker(
		WithRepositories(
			newMockQueueRepository(),
			newMockMessageRepositoryForWorker(),
			newMockSubscriptionRepositoryForWorker(),
			newMockDLQRepository(),
		),
		WithLogger(&NoopLogger{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TransmitterProvider is required")
}

// TestNewQueueWorker_MissingLogger verifies that missing Logger returns an error.
func TestNewQueueWorker_MissingLogger(t *testing.T) {
	_, err := NewQueueWorker(
		WithRepositories(
			newMockQueueRepository(),
			newMockMessageRepositoryForWorker(),
			newMockSubscriptionRepositoryForWorker(),
			newMockDLQRepository(),
		),
		WithDelivery(
			&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
			&mockDeliveryGateway{},
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Logger is required")
}

// TestNewQueueWorker_NilRepoInOption verifies that passing nil to WithRepositories returns an error.
func TestNewQueueWorker_NilRepoInOption(t *testing.T) {
	_, err := NewQueueWorker(
		WithRepositories(nil, nil, nil, nil),
		WithDelivery(
			&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
			&mockDeliveryGateway{},
		),
		WithLogger(&NoopLogger{}),
	)
	require.Error(t, err)
}

// TestNewQueueWorker_NilDeliveryInOption verifies that passing nil to WithDelivery returns an error.
func TestNewQueueWorker_NilDeliveryInOption(t *testing.T) {
	_, err := NewQueueWorker(
		WithRepositories(
			newMockQueueRepository(),
			newMockMessageRepositoryForWorker(),
			newMockSubscriptionRepositoryForWorker(),
			newMockDLQRepository(),
		),
		WithDelivery(nil, nil),
		WithLogger(&NoopLogger{}),
	)
	require.Error(t, err)
}

// TestNewQueueWorker_CustomBatchSize verifies that custom batch size is accepted.
func TestNewQueueWorker_CustomBatchSize(t *testing.T) {
	w, err := NewQueueWorker(
		WithRepositories(
			newMockQueueRepository(),
			newMockMessageRepositoryForWorker(),
			newMockSubscriptionRepositoryForWorker(),
			newMockDLQRepository(),
		),
		WithDelivery(
			&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
			&mockDeliveryGateway{},
		),
		WithLogger(&NoopLogger{}),
		WithBatchSize(50),
	)
	require.NoError(t, err)
	assert.NotNil(t, w)
	assert.Equal(t, 50, w.batchSize)
}

// TestNewQueueWorker_InvalidBatchSize verifies that a zero or negative batch size returns an error.
func TestNewQueueWorker_InvalidBatchSize(t *testing.T) {
	_, err := NewQueueWorker(
		WithRepositories(
			newMockQueueRepository(),
			newMockMessageRepositoryForWorker(),
			newMockSubscriptionRepositoryForWorker(),
			newMockDLQRepository(),
		),
		WithDelivery(
			&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
			&mockDeliveryGateway{},
		),
		WithLogger(&NoopLogger{}),
		WithBatchSize(0),
	)
	require.Error(t, err)
}

// --- ProcessPendingItems tests ---

// TestProcessPendingItems_NoPendingItems verifies that no items returns 0 processed count.
func TestProcessPendingItems_NoPendingItems(t *testing.T) {
	qr := newMockQueueRepository() // empty
	w := buildWorker(t, qr,
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	count, err := w.ProcessPendingItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestProcessPendingItems_RepoError verifies that a repository error is propagated.
func TestProcessPendingItems_RepoError(t *testing.T) {
	qr := newMockQueueRepository()
	qr.pendingErr = errors.New("database unreachable")

	w := buildWorker(t, qr,
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	_, err := w.ProcessPendingItems(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find pending items")
}

// TestProcessPendingItems_SuccessfulDelivery verifies that a pending item is delivered and marked sent.
func TestProcessPendingItems_SuccessfulDelivery(t *testing.T) {
	qr := newMockQueueRepository()
	mr := newMockMessageRepositoryForWorker()
	sr := newMockSubscriptionRepositoryForWorker()

	// Set up message, subscription, and queue item
	msg := model.Message{ID: 1, TopicID: 10, Identifier: "user-123", Data: `{"userId":42}`, CreatedAt: time.Now()}
	mr.messages[1] = msg

	sub := model.Subscription{ID: 1, SubscriberID: 100, TopicID: 10, Identifier: "user-123", IsActive: true}
	sr.subscriptions[1] = sub

	item := model.NewQueue(1, 1)
	item.ID = 1
	item.ExpiresAt = time.Now().Add(24 * time.Hour)
	qr.pendingItems = []model.Queue{item}

	gateway := &mockDeliveryGateway{}
	provider := &mockTransmitterProvider{callbackURL: "https://example.com/webhook"}

	w := buildWorker(t, qr, mr, sr, newMockDLQRepository(), provider, gateway)

	count, err := w.ProcessPendingItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, 1, gateway.deliverCalls)
}

// TestProcessPendingItems_DeliveryFailureSchedulesRetry verifies that a delivery failure marks
// the queue item as failed and schedules a retry.
func TestProcessPendingItems_DeliveryFailureSchedulesRetry(t *testing.T) {
	qr := newMockQueueRepository()
	mr := newMockMessageRepositoryForWorker()
	sr := newMockSubscriptionRepositoryForWorker()

	msg := model.Message{ID: 1, TopicID: 10, Identifier: "user-123", Data: `{}`, CreatedAt: time.Now()}
	mr.messages[1] = msg

	sub := model.Subscription{ID: 1, SubscriberID: 100, TopicID: 10, Identifier: "user-123", IsActive: true}
	sr.subscriptions[1] = sub

	item := model.NewQueue(1, 1)
	item.ID = 1
	item.ExpiresAt = time.Now().Add(24 * time.Hour)
	qr.pendingItems = []model.Queue{item}

	gateway := &mockDeliveryGateway{deliverErr: errors.New("connection refused")}
	provider := &mockTransmitterProvider{callbackURL: "https://example.com/webhook"}

	w := buildWorker(t, qr, mr, sr, newMockDLQRepository(), provider, gateway)

	// ProcessPendingItems returns 0 because delivery failed
	count, err := w.ProcessPendingItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// The item should have been saved with failed status
	require.Len(t, qr.savedItems, 1)
	assert.Equal(t, model.QueueStatusFailed, qr.savedItems[0].Status)
	assert.Equal(t, 1, qr.savedItems[0].AttemptCount)
	assert.True(t, qr.savedItems[0].NextRetryAt.Valid)
}

// TestProcessPendingItems_MaxAttemptsExceeded verifies that an item at max attempts is skipped.
func TestProcessPendingItems_MaxAttemptsExceeded(t *testing.T) {
	qr := newMockQueueRepository()
	mr := newMockMessageRepositoryForWorker()
	sr := newMockSubscriptionRepositoryForWorker()

	msg := model.Message{ID: 1, TopicID: 10, Identifier: "user-123", Data: `{}`, CreatedAt: time.Now()}
	mr.messages[1] = msg
	sub := model.Subscription{ID: 1, SubscriberID: 100, TopicID: 10, Identifier: "user-123", IsActive: true}
	sr.subscriptions[1] = sub

	item := model.NewQueue(1, 1)
	item.ID = 1
	item.AttemptCount = 10 // equals MaxAttempts (10)
	item.ExpiresAt = time.Now().Add(24 * time.Hour)
	qr.pendingItems = []model.Queue{item}

	gateway := &mockDeliveryGateway{}
	w := buildWorker(t, qr, mr, sr, newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		gateway,
	)

	count, err := w.ProcessPendingItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	// Gateway should not be called because CanAttemptDelivery returns error
	assert.Equal(t, 0, gateway.deliverCalls)
}

// --- ProcessRetryableItems tests ---

// TestProcessRetryableItems_NoPendingItems verifies that empty retryable queue returns 0.
func TestProcessRetryableItems_NoPendingItems(t *testing.T) {
	w := buildWorker(t,
		newMockQueueRepository(),
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	count, err := w.ProcessRetryableItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestProcessRetryableItems_SuccessOnRetry verifies that a failed item is successfully
// retried when ready.
func TestProcessRetryableItems_SuccessOnRetry(t *testing.T) {
	qr := newMockQueueRepository()
	mr := newMockMessageRepositoryForWorker()
	sr := newMockSubscriptionRepositoryForWorker()

	msg := model.Message{ID: 2, TopicID: 10, Identifier: "user-456", Data: `{"retry":true}`, CreatedAt: time.Now()}
	mr.messages[2] = msg

	sub := model.Subscription{ID: 2, SubscriberID: 200, TopicID: 10, Identifier: "user-456", IsActive: true}
	sr.subscriptions[2] = sub

	// Item already failed once, retry time has passed
	item := model.NewQueue(2, 2)
	item.ID = 2
	item.Status = model.QueueStatusFailed
	item.AttemptCount = 1
	item.NextRetryAt = sql.NullTime{Time: time.Now().Add(-time.Minute), Valid: true}
	item.ExpiresAt = time.Now().Add(24 * time.Hour)
	qr.retryItems = []model.Queue{item}

	gateway := &mockDeliveryGateway{}
	w := buildWorker(t, qr, mr, sr, newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		gateway,
	)

	count, err := w.ProcessRetryableItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, 1, gateway.deliverCalls)
}

// --- DLQ tests ---

// TestProcessPendingItems_MovesToDLQAfterThreshold verifies that an item exceeding the DLQ
// threshold is moved to the Dead Letter Queue.
func TestProcessPendingItems_MovesToDLQAfterThreshold(t *testing.T) {
	qr := newMockQueueRepository()
	mr := newMockMessageRepositoryForWorker()
	sr := newMockSubscriptionRepositoryForWorker()
	dlqr := newMockDLQRepository()

	msg := model.Message{ID: 3, TopicID: 10, Identifier: "dlq-test", Data: `{}`, CreatedAt: time.Now()}
	mr.messages[3] = msg

	sub := model.Subscription{ID: 3, SubscriberID: 300, TopicID: 10, Identifier: "dlq-test", IsActive: true}
	sr.subscriptions[3] = sub

	// AttemptCount is already at DLQ threshold - 1 so next failure will trigger DLQ
	fastStrategy := retry.Strategy{
		MaxAttempts:     10,
		BaseDelay:       time.Second,
		MaxDelay:        time.Minute,
		ExponentialBase: 2.0,
		DLQThreshold:    1, // Very low threshold for test: 1 failure → DLQ
	}

	item := model.NewQueue(3, 3)
	item.ID = 3
	item.ExpiresAt = time.Now().Add(24 * time.Hour)
	qr.pendingItems = []model.Queue{item}

	gateway := &mockDeliveryGateway{deliverErr: errors.New("permanent error")}
	provider := &mockTransmitterProvider{callbackURL: "https://example.com/webhook"}

	w, err := NewQueueWorker(
		WithRepositories(qr, mr, sr, dlqr),
		WithDelivery(provider, gateway),
		WithLogger(&NoopLogger{}),
		WithRetryStrategy(fastStrategy),
	)
	require.NoError(t, err)

	_, processErr := w.ProcessPendingItems(context.Background())
	require.NoError(t, processErr)

	// DLQ should contain exactly one entry
	require.Len(t, dlqr.savedDLQ, 1)
	assert.Equal(t, int64(3), dlqr.savedDLQ[0].MessageID)
	assert.Contains(t, dlqr.savedDLQ[0].FailureReason, "Max retry attempts exceeded")
}

// --- CleanupExpiredItems tests ---

// TestCleanupExpiredItems_NoExpiredItems verifies that an empty expired list returns 0 deleted.
func TestCleanupExpiredItems_NoExpiredItems(t *testing.T) {
	w := buildWorker(t,
		newMockQueueRepository(),
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	deleted, err := w.CleanupExpiredItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

// TestCleanupExpiredItems_DeletesExpiredItems verifies that expired items are removed from the queue.
func TestCleanupExpiredItems_DeletesExpiredItems(t *testing.T) {
	qr := newMockQueueRepository()

	expired1 := model.NewQueue(1, 1)
	expired1.ID = 10
	expired1.ExpiresAt = time.Now().Add(-2 * time.Hour)

	expired2 := model.NewQueue(2, 2)
	expired2.ID = 11
	expired2.ExpiresAt = time.Now().Add(-1 * time.Hour)

	qr.expiredItems = []model.Queue{expired1, expired2}

	w := buildWorker(t, qr,
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	deleted, err := w.CleanupExpiredItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)
	assert.Len(t, qr.deletedItems, 2)
}

// TestCleanupExpiredItems_DeleteErrorContinues verifies that a delete error for one item
// does not stop cleanup of subsequent items.
func TestCleanupExpiredItems_DeleteErrorContinues(t *testing.T) {
	qr := newMockQueueRepository()
	qr.deleteErr = errors.New("lock timeout")

	expired := model.NewQueue(1, 1)
	expired.ID = 10
	expired.ExpiresAt = time.Now().Add(-1 * time.Hour)
	qr.expiredItems = []model.Queue{expired}

	w := buildWorker(t, qr,
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	deleted, err := w.CleanupExpiredItems(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, deleted) // error prevented deletion
}

// TestCleanupExpiredItems_RepoError verifies that a repo error is propagated.
func TestCleanupExpiredItems_RepoError(t *testing.T) {
	qr := newMockQueueRepository()
	qr.expiredErr = errors.New("expired query failed")

	w := buildWorker(t, qr,
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	_, err := w.CleanupExpiredItems(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find expired items")
}

// --- Run tests ---

// TestRun_CancelContextStops verifies that canceling the context stops the worker loop.
func TestRun_CancelContextStops(t *testing.T) {
	w := buildWorker(t,
		newMockQueueRepository(),
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		w.Run(ctx, 50*time.Millisecond)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Worker stopped as expected
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

// --- GetRetrySchedule tests ---

// TestGetRetrySchedule_ReturnsNonEmpty verifies the retry schedule description is non-empty.
func TestGetRetrySchedule_ReturnsNonEmpty(t *testing.T) {
	w := buildWorker(t,
		newMockQueueRepository(),
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		newMockDLQRepository(),
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	schedule := w.GetRetrySchedule()
	assert.NotEmpty(t, schedule)
	assert.Contains(t, schedule, "Retry Schedule")
}

// --- GetDLQStats tests ---

// TestGetDLQStats_ReturnsStats verifies DLQ stats are returned correctly.
func TestGetDLQStats_ReturnsStats(t *testing.T) {
	dlqr := newMockDLQRepository()
	dlqr.items[1] = model.DeadLetterQueue{ID: 1, MessageID: 10}
	dlqr.items[2] = model.DeadLetterQueue{ID: 2, MessageID: 11}

	w := buildWorker(t,
		newMockQueueRepository(),
		newMockMessageRepositoryForWorker(),
		newMockSubscriptionRepositoryForWorker(),
		dlqr,
		&mockTransmitterProvider{callbackURL: "https://example.com/webhook"},
		&mockDeliveryGateway{},
	)

	stats, err := w.GetDLQStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalItems)
}

// --- Notification integration tests ---

// TestProcessPendingItems_DeliveryFailureNotifiesThroughNotificationService verifies that
// a delivery failure triggers the notification service.
func TestProcessPendingItems_DeliveryFailureNotifiesThroughNotificationService(t *testing.T) {
	qr := newMockQueueRepository()
	mr := newMockMessageRepositoryForWorker()
	sr := newMockSubscriptionRepositoryForWorker()

	msg := model.Message{ID: 5, TopicID: 10, Identifier: "notify-test", Data: `{}`, CreatedAt: time.Now()}
	mr.messages[5] = msg

	sub := model.Subscription{ID: 5, SubscriberID: 500, TopicID: 10, Identifier: "notify-test", IsActive: true}
	sr.subscriptions[5] = sub

	item := model.NewQueue(5, 5)
	item.ID = 5
	item.ExpiresAt = time.Now().Add(24 * time.Hour)
	qr.pendingItems = []model.Queue{item}

	notifier := &mockNotificationService{}
	gateway := &mockDeliveryGateway{deliverErr: errors.New("service unavailable")}

	w, err := NewQueueWorker(
		WithRepositories(qr, mr, sr, newMockDLQRepository()),
		WithDelivery(&mockTransmitterProvider{callbackURL: "https://example.com/webhook"}, gateway),
		WithLogger(&NoopLogger{}),
		WithNotifications(notifier),
	)
	require.NoError(t, err)

	_, processErr := w.ProcessPendingItems(context.Background())
	require.NoError(t, processErr)

	assert.Equal(t, 1, notifier.deliveryFailureCalls)
}

// mockNotificationService implements NotificationService for testing.
type mockNotificationService struct {
	deliveryFailureCalls int
	dlqAddedCalls        int
}

func (m *mockNotificationService) NotifyDLQItemAdded(_ context.Context, _ model.DeadLetterQueue) error {
	m.dlqAddedCalls++
	return nil
}

func (m *mockNotificationService) NotifyDeliveryFailure(_ context.Context, _ *model.Queue, _ error) error {
	m.deliveryFailureCalls++
	return nil
}

func (m *mockNotificationService) NotifySubscriptionCreated(_ context.Context, _ model.Subscription) error {
	return nil
}

func (m *mockNotificationService) NotifySubscriptionDeactivated(_ context.Context, _ model.Subscription) error {
	return nil
}
