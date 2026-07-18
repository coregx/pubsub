//go:build cgo

// Integration tests for all Relica repository adapters.
//
// These tests require CGO (for mattn/go-sqlite3 which wraps the standard SQLite C library).
// The standard C library is used because its column-name matching is case-insensitive at
// the SQL parser level, which is required for these adapters: Relica's StructToMap() emits
// PascalCase column names for struct fields without db:"" tags (used in INSERT), while the
// adapter code uses snake_case in WHERE predicates. The C library handles this mismatch;
// pure-Go modernc.org/sqlite does not.
//
// The tests run automatically on Linux/macOS CI where CGO+gcc are available.
// On Windows without a C toolchain, the build tag excludes this file from compilation.
package relica

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/model"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDB is the shared in-memory SQLite database used by all integration tests.
var testDB *sql.DB

// TestMain sets up the shared in-memory SQLite database and runs all tests.
func TestMain(m *testing.M) {
	var err error
	// cache=shared allows multiple connections to share the same in-memory DB.
	testDB, err = sql.Open("sqlite3", "file::memory:?cache=shared&_fk=off")
	if err != nil {
		panic("failed to open sqlite3: " + err.Error())
	}
	createTables(testDB)
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

// createTables creates all tables needed by integration tests using SQLite-compatible DDL.
//
// SQLite compatibility notes for models without db:"id" tags:
//
//  1. Relica's StructToMap() uses the Go field name when no db:"" tag is present.
//     For a field named "ID" this produces map key "ID" (uppercase), not "id".
//
//  2. Relica's Insert() attempts to remove the zero-valued PK via:
//     delete(filteredMap, pkInfo.Columns[0])  // pkInfo.Columns[0] == "id" (lowercase)
//     Because the map key is "ID" (uppercase) this delete is a no-op: the field
//     "ID"=0 remains in the map and is sent to SQLite explicitly.
//
//  3. In SQLite with "INTEGER PRIMARY KEY" the rowid alias behavior means that
//     inserting id=0 is allowed (rowid 0), and last_insert_rowid() returns 0,
//     so Relica's auto-populate sets m.ID=0 again — breaking all subsequent operations.
//
//  4. Workaround: declare id as INT (not INTEGER) PRIMARY KEY.
//     "INT PRIMARY KEY" is NOT a rowid alias in SQLite, so the auto-assigned rowid
//     is independent of the id column. An AFTER INSERT trigger then syncs id to
//     last_insert_rowid() when id was 0. last_insert_rowid() returns the new rowid
//     (e.g., 1, 2, 3…), which Relica writes back to m.ID via populatePrimaryKey.
//     The UPDATE trigger ensures the stored id matches the returned value.
//
//  5. DeadLetterQueue.ID also has no db:"" tag (only json:"id"), so the same INT+trigger
//     pattern applies. All other fields do have explicit db:"…" tags; only ID is untagged.
func createTables(db *sql.DB) {
	// pubsub_publisher — model.Publisher
	mustExec(db, `CREATE TABLE IF NOT EXISTS pubsub_publisher (
		id             INT NOT NULL DEFAULT 0,
		publisher_code TEXT NOT NULL DEFAULT '',
		name           TEXT NOT NULL DEFAULT '',
		description    TEXT NOT NULL DEFAULT '',
		is_active      INTEGER NOT NULL DEFAULT 1,
		created_at     DATETIME NOT NULL,
		PRIMARY KEY    (id)
	)`)
	// Sync id ← rowid when a zero id is inserted.
	mustExec(db, `CREATE TRIGGER IF NOT EXISTS trg_publisher_id
		AFTER INSERT ON pubsub_publisher WHEN NEW.id = 0
		BEGIN
			UPDATE pubsub_publisher SET id = last_insert_rowid()
			WHERE rowid = last_insert_rowid();
		END`)

	// pubsub_subscriber — model.Subscriber
	mustExec(db, `CREATE TABLE IF NOT EXISTS pubsub_subscriber (
		id          INT NOT NULL DEFAULT 0,
		client_id   INTEGER NOT NULL DEFAULT 0,
		name        TEXT NOT NULL DEFAULT '',
		webhook_url TEXT NOT NULL DEFAULT '',
		is_active   INTEGER NOT NULL DEFAULT 1,
		created_at  DATETIME NOT NULL,
		PRIMARY KEY (id)
	)`)
	mustExec(db, `CREATE TRIGGER IF NOT EXISTS trg_subscriber_id
		AFTER INSERT ON pubsub_subscriber WHEN NEW.id = 0
		BEGIN
			UPDATE pubsub_subscriber SET id = last_insert_rowid()
			WHERE rowid = last_insert_rowid();
		END`)

	// pubsub_topic — model.Topic
	mustExec(db, `CREATE TABLE IF NOT EXISTS pubsub_topic (
		id          INT NOT NULL DEFAULT 0,
		topic_code  TEXT NOT NULL DEFAULT '',
		name        TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		is_active   INTEGER NOT NULL DEFAULT 1,
		created_at  DATETIME NOT NULL,
		PRIMARY KEY (id)
	)`)
	mustExec(db, `CREATE TRIGGER IF NOT EXISTS trg_topic_id
		AFTER INSERT ON pubsub_topic WHEN NEW.id = 0
		BEGIN
			UPDATE pubsub_topic SET id = last_insert_rowid()
			WHERE rowid = last_insert_rowid();
		END`)

	// pubsub_message — model.Message.
	// Untagged fields use Go names: TopicID, Identifier, Data, CreatedAt.
	mustExec(db, `CREATE TABLE IF NOT EXISTS pubsub_message (
		id         INT NOT NULL DEFAULT 0,
		TopicID    INTEGER NOT NULL DEFAULT 0,
		Identifier TEXT NOT NULL DEFAULT '',
		Data       TEXT NOT NULL DEFAULT '',
		CreatedAt  DATETIME NOT NULL,
		PRIMARY KEY (id)
	)`)
	mustExec(db, `CREATE TRIGGER IF NOT EXISTS trg_message_id
		AFTER INSERT ON pubsub_message WHEN NEW.id = 0
		BEGIN
			UPDATE pubsub_message SET id = last_insert_rowid()
			WHERE rowid = last_insert_rowid();
		END`)

	// pubsub_subscription — model.Subscription.
	// Untagged fields use Go names: ID, SubscriberID, TopicID, Identifier, IsActive,
	// CreatedAt, DeletedAt.
	mustExec(db, `CREATE TABLE IF NOT EXISTS pubsub_subscription (
		id           INT NOT NULL DEFAULT 0,
		SubscriberID INTEGER NOT NULL DEFAULT 0,
		TopicID      INTEGER NOT NULL DEFAULT 0,
		Identifier   TEXT NOT NULL DEFAULT '',
		IsActive     INTEGER NOT NULL DEFAULT 1,
		CreatedAt    DATETIME NOT NULL,
		DeletedAt    DATETIME,
		PRIMARY KEY  (id)
	)`)
	mustExec(db, `CREATE TRIGGER IF NOT EXISTS trg_subscription_id
		AFTER INSERT ON pubsub_subscription WHEN NEW.id = 0
		BEGIN
			UPDATE pubsub_subscription SET id = last_insert_rowid()
			WHERE rowid = last_insert_rowid();
		END`)

	// pubsub_queue — model.Queue.
	// Tagged (snake_case): status, attempt_count, last_attempt_at, next_retry_at,
	//   last_error, expires_at, sequence_number, operation_timestamp.
	// Untagged (Go names): SubscriptionID, MessageID, RetryAt, IsComplete,
	//   CompletedAt, CreatedAt.
	mustExec(db, `CREATE TABLE IF NOT EXISTS pubsub_queue (
		id                  INT NOT NULL DEFAULT 0,
		SubscriptionID      INTEGER NOT NULL DEFAULT 0,
		MessageID           INTEGER NOT NULL DEFAULT 0,
		status              TEXT NOT NULL DEFAULT 'pending',
		attempt_count       INTEGER NOT NULL DEFAULT 0,
		last_attempt_at     DATETIME,
		next_retry_at       DATETIME,
		last_error          TEXT,
		expires_at          DATETIME NOT NULL,
		sequence_number     INTEGER NOT NULL DEFAULT 0,
		operation_timestamp DATETIME NOT NULL,
		RetryAt             DATETIME,
		IsComplete          INTEGER NOT NULL DEFAULT 0,
		CompletedAt         DATETIME,
		CreatedAt           DATETIME NOT NULL,
		PRIMARY KEY         (id)
	)`)
	mustExec(db, `CREATE TRIGGER IF NOT EXISTS trg_queue_id
		AFTER INSERT ON pubsub_queue WHEN NEW.id = 0
		BEGIN
			UPDATE pubsub_queue SET id = last_insert_rowid()
			WHERE rowid = last_insert_rowid();
		END`)

	// pubsub_dead_letter_queue — model.DeadLetterQueue (adapter uses this table name, not "dlq").
	// DeadLetterQueue.ID has no db:"" tag, so StructToMap emits key "ID" (PascalCase).
	// Same INT+trigger workaround applies as for all other tables.
	mustExec(db, `CREATE TABLE IF NOT EXISTS pubsub_dead_letter_queue (
		id                  INT NOT NULL DEFAULT 0,
		subscription_id     INTEGER NOT NULL DEFAULT 0,
		message_id          INTEGER NOT NULL DEFAULT 0,
		original_queue_id   INTEGER NOT NULL DEFAULT 0,
		attempt_count       INTEGER NOT NULL DEFAULT 0,
		last_error          TEXT NOT NULL DEFAULT '',
		failure_reason      TEXT NOT NULL DEFAULT '',
		first_attempt_at    DATETIME NOT NULL,
		last_attempt_at     DATETIME NOT NULL,
		moved_to_dlq_at     DATETIME NOT NULL,
		message_data        TEXT NOT NULL DEFAULT '',
		callback_url        TEXT NOT NULL DEFAULT '',
		is_resolved         INTEGER NOT NULL DEFAULT 0,
		resolved_at         DATETIME,
		resolved_by         TEXT NOT NULL DEFAULT '',
		resolution_note     TEXT NOT NULL DEFAULT '',
		created_at          DATETIME NOT NULL,
		PRIMARY KEY         (id)
	)`)
	mustExec(db, `CREATE TRIGGER IF NOT EXISTS trg_dlq_id
		AFTER INSERT ON pubsub_dead_letter_queue WHEN NEW.id = 0
		BEGIN
			UPDATE pubsub_dead_letter_queue SET id = last_insert_rowid()
			WHERE rowid = last_insert_rowid();
		END`)
}

// mustExec panics if a DDL statement fails — test setup failure is fatal.
func mustExec(db *sql.DB, query string) {
	if _, err := db.Exec(query); err != nil {
		panic("createTables: " + err.Error() + "\nQuery: " + query)
	}
}

// ctx returns a background context for use in tests.
func ctx() context.Context {
	return context.Background()
}

// truncate clears a table between tests to ensure isolation.
func truncate(t *testing.T, table string) {
	t.Helper()
	_, err := testDB.Exec("DELETE FROM " + table)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// PublisherRepository tests
// ---------------------------------------------------------------------------

func TestPublisherRepository_SaveAndLoad(t *testing.T) {
	truncate(t, "pubsub_publisher")
	repo := NewPublisherRepository(testDB, "sqlite3")

	pub := model.NewPublisher("svc-001", "Order Service", "Publishes order events")
	saved, err := repo.Save(ctx(), pub)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0), "ID must be auto-populated after insert")
	assert.Equal(t, "svc-001", saved.Code)
	assert.Equal(t, "Order Service", saved.Name)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, loaded.ID)
	assert.Equal(t, "svc-001", loaded.Code)
	assert.Equal(t, "Order Service", loaded.Name)
	assert.True(t, loaded.IsActive)
}

func TestPublisherRepository_Load_NotFound(t *testing.T) {
	truncate(t, "pubsub_publisher")
	repo := NewPublisherRepository(testDB, "sqlite3")

	_, err := repo.Load(ctx(), 9999)
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err), "expected ErrNoData for missing publisher")
}

func TestPublisherRepository_Update(t *testing.T) {
	truncate(t, "pubsub_publisher")
	repo := NewPublisherRepository(testDB, "sqlite3")

	pub := model.NewPublisher("svc-update", "Old Name", "Old desc")
	saved, err := repo.Save(ctx(), pub)
	require.NoError(t, err)

	saved.Name = "New Name"
	saved.Description = "New desc"
	updated, err := repo.Save(ctx(), saved)
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", loaded.Name)
	assert.Equal(t, "New desc", loaded.Description)
}

func TestPublisherRepository_GetByPublisherCode(t *testing.T) {
	truncate(t, "pubsub_publisher")
	repo := NewPublisherRepository(testDB, "sqlite3")

	pub := model.NewPublisher("code-xyz", "XYZ Publisher", "desc")
	saved, err := repo.Save(ctx(), pub)
	require.NoError(t, err)

	found, err := repo.GetByPublisherCode(ctx(), "code-xyz")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, found.ID)
	assert.Equal(t, "code-xyz", found.Code)
}

func TestPublisherRepository_GetByPublisherCode_NotFound(t *testing.T) {
	truncate(t, "pubsub_publisher")
	repo := NewPublisherRepository(testDB, "sqlite3")

	_, err := repo.GetByPublisherCode(ctx(), "does-not-exist")
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

// ---------------------------------------------------------------------------
// SubscriberRepository tests
// ---------------------------------------------------------------------------

func TestSubscriberRepository_SaveAndLoad(t *testing.T) {
	truncate(t, "pubsub_subscriber")
	repo := NewSubscriberRepository(testDB, "sqlite3")

	sub := model.NewSubscriber(42, "Acme Subscriber", "https://acme.example.com/hook")
	saved, err := repo.Save(ctx(), sub)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0))
	assert.Equal(t, int64(42), saved.ClientID)
	assert.Equal(t, "Acme Subscriber", saved.Name)
	assert.Equal(t, "https://acme.example.com/hook", saved.WebhookURL)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, loaded.ID)
	assert.Equal(t, "https://acme.example.com/hook", loaded.WebhookURL)
	assert.True(t, loaded.IsActive)
}

func TestSubscriberRepository_Load_NotFound(t *testing.T) {
	truncate(t, "pubsub_subscriber")
	repo := NewSubscriberRepository(testDB, "sqlite3")

	_, err := repo.Load(ctx(), 9999)
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

func TestSubscriberRepository_Update(t *testing.T) {
	truncate(t, "pubsub_subscriber")
	repo := NewSubscriberRepository(testDB, "sqlite3")

	sub := model.NewSubscriber(1, "Old Sub", "https://old.example.com/hook")
	saved, err := repo.Save(ctx(), sub)
	require.NoError(t, err)

	saved.Name = "Updated Sub"
	saved.WebhookURL = "https://new.example.com/hook"
	_, err = repo.Save(ctx(), saved)
	require.NoError(t, err)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Sub", loaded.Name)
	assert.Equal(t, "https://new.example.com/hook", loaded.WebhookURL)
}

func TestSubscriberRepository_FindByClientID(t *testing.T) {
	truncate(t, "pubsub_subscriber")
	repo := NewSubscriberRepository(testDB, "sqlite3")

	sub := model.NewSubscriber(777, "Client Sub", "https://client.example.com/hook")
	saved, err := repo.Save(ctx(), sub)
	require.NoError(t, err)

	found, err := repo.FindByClientID(ctx(), 777)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, found.ID)
	assert.Equal(t, int64(777), found.ClientID)
}

func TestSubscriberRepository_FindByClientID_NotFound(t *testing.T) {
	truncate(t, "pubsub_subscriber")
	repo := NewSubscriberRepository(testDB, "sqlite3")

	_, err := repo.FindByClientID(ctx(), 9999)
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

func TestSubscriberRepository_FindByName(t *testing.T) {
	truncate(t, "pubsub_subscriber")
	repo := NewSubscriberRepository(testDB, "sqlite3")

	sub := model.NewSubscriber(1, "Unique Sub Name", "https://sub.example.com/hook")
	saved, err := repo.Save(ctx(), sub)
	require.NoError(t, err)

	found, err := repo.FindByName(ctx(), "Unique Sub Name")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, found.ID)
}

func TestSubscriberRepository_FindByName_NotFound(t *testing.T) {
	truncate(t, "pubsub_subscriber")
	repo := NewSubscriberRepository(testDB, "sqlite3")

	_, err := repo.FindByName(ctx(), "ghost subscriber")
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

// ---------------------------------------------------------------------------
// TopicRepository tests
// ---------------------------------------------------------------------------

func TestTopicRepository_SaveAndLoad(t *testing.T) {
	truncate(t, "pubsub_topic")
	repo := NewTopicRepository(testDB, "sqlite3")

	topic := model.NewTopic("user.signup", "User Signup", "Fired when a new user registers")
	saved, err := repo.Save(ctx(), topic)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0))
	assert.Equal(t, "user.signup", saved.Code)
	assert.Equal(t, "User Signup", saved.Name)
	assert.True(t, saved.IsActive)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, loaded.ID)
	assert.Equal(t, "user.signup", loaded.Code)
}

func TestTopicRepository_Load_NotFound(t *testing.T) {
	truncate(t, "pubsub_topic")
	repo := NewTopicRepository(testDB, "sqlite3")

	_, err := repo.Load(ctx(), 9999)
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

func TestTopicRepository_Update(t *testing.T) {
	truncate(t, "pubsub_topic")
	repo := NewTopicRepository(testDB, "sqlite3")

	topic := model.NewTopic("order.created", "Order Created", "Old desc")
	saved, err := repo.Save(ctx(), topic)
	require.NoError(t, err)

	saved.Name = "Order Created V2"
	saved.Description = "Updated description"
	_, err = repo.Save(ctx(), saved)
	require.NoError(t, err)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "Order Created V2", loaded.Name)
	assert.Equal(t, "Updated description", loaded.Description)
}

func TestTopicRepository_GetByTopicCode(t *testing.T) {
	truncate(t, "pubsub_topic")
	repo := NewTopicRepository(testDB, "sqlite3")

	topic := model.NewTopic("payment.received", "Payment Received", "desc")
	saved, err := repo.Save(ctx(), topic)
	require.NoError(t, err)

	found, err := repo.GetByTopicCode(ctx(), "payment.received")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, found.ID)
	assert.Equal(t, "payment.received", found.Code)
}

func TestTopicRepository_GetByTopicCode_NotFound(t *testing.T) {
	truncate(t, "pubsub_topic")
	repo := NewTopicRepository(testDB, "sqlite3")

	_, err := repo.GetByTopicCode(ctx(), "non.existent.topic")
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

// ---------------------------------------------------------------------------
// MessageRepository tests
// ---------------------------------------------------------------------------

func TestMessageRepository_SaveAndLoad(t *testing.T) {
	truncate(t, "pubsub_message")
	repo := NewMessageRepository(testDB, "sqlite3")

	msg := model.NewMessage(10, "user-999", `{"action":"created"}`)
	saved, err := repo.Save(ctx(), msg)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0))
	assert.Equal(t, int64(10), saved.TopicID)
	assert.Equal(t, "user-999", saved.Identifier)
	assert.Equal(t, `{"action":"created"}`, saved.Data)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, loaded.ID)
	assert.Equal(t, `{"action":"created"}`, loaded.Data)
}

func TestMessageRepository_Load_NotFound(t *testing.T) {
	truncate(t, "pubsub_message")
	repo := NewMessageRepository(testDB, "sqlite3")

	_, err := repo.Load(ctx(), 9999)
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

func TestMessageRepository_Update(t *testing.T) {
	truncate(t, "pubsub_message")
	repo := NewMessageRepository(testDB, "sqlite3")

	msg := model.NewMessage(1, "item-1", `{"v":1}`)
	saved, err := repo.Save(ctx(), msg)
	require.NoError(t, err)

	saved.Data = `{"v":2}`
	updated, err := repo.Save(ctx(), saved)
	require.NoError(t, err)
	assert.Equal(t, `{"v":2}`, updated.Data)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, `{"v":2}`, loaded.Data)
}

func TestMessageRepository_Delete(t *testing.T) {
	truncate(t, "pubsub_message")
	repo := NewMessageRepository(testDB, "sqlite3")

	msg := model.NewMessage(1, "del-1", `{}`)
	saved, err := repo.Save(ctx(), msg)
	require.NoError(t, err)

	err = repo.Delete(ctx(), saved)
	require.NoError(t, err)

	_, err = repo.Load(ctx(), saved.ID)
	assert.True(t, pubsub.IsNoData(err), "deleted message must not be found")
}

func TestMessageRepository_FindOutdatedMessages(t *testing.T) {
	truncate(t, "pubsub_message")
	repo := NewMessageRepository(testDB, "sqlite3")

	// Insert a message with a past CreatedAt by saving then raw-updating.
	msg := model.NewMessage(1, "old-msg", `{}`)
	saved, err := repo.Save(ctx(), msg)
	require.NoError(t, err)

	// Backdate the message to 10 days ago via direct SQL.
	oldTime := time.Now().AddDate(0, 0, -10).UTC().Format("2006-01-02 15:04:05")
	_, err = testDB.Exec("UPDATE pubsub_message SET CreatedAt = ? WHERE id = ?", oldTime, saved.ID)
	require.NoError(t, err)

	// Insert a recent message (CreatedAt = now, default).
	_, err = repo.Save(ctx(), model.NewMessage(1, "new-msg", `{}`))
	require.NoError(t, err)

	outdated, err := repo.FindOutdatedMessages(ctx(), 7)
	require.NoError(t, err)
	assert.Len(t, outdated, 1)
	assert.Equal(t, "old-msg", outdated[0].Identifier)
}

func TestMessageRepository_FindOutdatedMessages_Empty(t *testing.T) {
	truncate(t, "pubsub_message")
	repo := NewMessageRepository(testDB, "sqlite3")

	// All messages are recent.
	_, err := repo.Save(ctx(), model.NewMessage(1, "new", `{}`))
	require.NoError(t, err)

	_, err = repo.FindOutdatedMessages(ctx(), 7)
	assert.True(t, pubsub.IsNoData(err), "expected ErrNoData when no outdated messages")
}

// ---------------------------------------------------------------------------
// SubscriptionRepository tests
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_SaveAndLoad(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	sub := model.NewSubscription(10, 20, "user-*", "https://hook.example.com")
	saved, err := repo.Save(ctx(), sub)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0))
	assert.Equal(t, int64(10), saved.SubscriberID)
	assert.Equal(t, int64(20), saved.TopicID)
	assert.Equal(t, "user-*", saved.Identifier)
	assert.True(t, saved.IsActive)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, loaded.ID)
	assert.True(t, loaded.IsActive)
}

func TestSubscriptionRepository_Load_NotFound(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	_, err := repo.Load(ctx(), 9999)
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

func TestSubscriptionRepository_Update(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	sub := model.NewSubscription(1, 1, "event-*", "")
	saved, err := repo.Save(ctx(), sub)
	require.NoError(t, err)

	saved.Identifier = "order-*"
	saved.IsActive = false
	updated, err := repo.Save(ctx(), saved)
	require.NoError(t, err)
	assert.Equal(t, "order-*", updated.Identifier)
	assert.False(t, updated.IsActive)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "order-*", loaded.Identifier)
	assert.False(t, loaded.IsActive)
}

func TestSubscriptionRepository_FindActive(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	active := model.NewSubscription(5, 1, "item-1", "")
	saved1, err := repo.Save(ctx(), active)
	require.NoError(t, err)

	inactive := model.NewSubscription(5, 1, "item-2", "")
	inactive.IsActive = false
	_, err = repo.Save(ctx(), inactive)
	require.NoError(t, err)

	results, err := repo.FindActive(ctx(), 5, "")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, saved1.ID, results[0].ID)
}

func TestSubscriptionRepository_FindActive_WithIdentifier(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	sub1 := model.NewSubscription(1, 1, "user-100", "")
	_, err := repo.Save(ctx(), sub1)
	require.NoError(t, err)

	sub2 := model.NewSubscription(1, 1, "user-200", "")
	_, err = repo.Save(ctx(), sub2)
	require.NoError(t, err)

	results, err := repo.FindActive(ctx(), 1, "user-100")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "user-100", results[0].Identifier)
}

func TestSubscriptionRepository_FindActive_NotFound(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	_, err := repo.FindActive(ctx(), 9999, "")
	assert.True(t, pubsub.IsNoData(err))
}

func TestSubscriptionRepository_List(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	for i := range 3 {
		s := model.NewSubscription(int64(i+1), 1, "id", "")
		_, err := repo.Save(ctx(), s)
		require.NoError(t, err)
	}

	// List by subscriber ID.
	results, err := repo.List(ctx(), pubsub.Filter{SubscriberID: 2})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, int64(2), results[0].SubscriberID)
}

func TestSubscriptionRepository_List_Empty(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	_, err := repo.List(ctx(), pubsub.Filter{SubscriberID: 999})
	assert.True(t, pubsub.IsNoData(err))
}

func TestSubscriptionRepository_FindAllActive(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	s1 := model.NewSubscription(1, 1, "a", "")
	_, err := repo.Save(ctx(), s1)
	require.NoError(t, err)

	s2 := model.NewSubscription(2, 1, "b", "")
	_, err = repo.Save(ctx(), s2)
	require.NoError(t, err)

	s3 := model.NewSubscription(3, 1, "c", "")
	s3.IsActive = false
	_, err = repo.Save(ctx(), s3)
	require.NoError(t, err)

	results, err := repo.FindAllActive(ctx())
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSubscriptionRepository_FindAllActive_Empty(t *testing.T) {
	truncate(t, "pubsub_subscription")
	repo := NewSubscriptionRepository(testDB, "sqlite3")

	_, err := repo.FindAllActive(ctx())
	assert.True(t, pubsub.IsNoData(err))
}

// ---------------------------------------------------------------------------
// QueueRepository tests
// ---------------------------------------------------------------------------

// insertQueue inserts a Queue row using NewQueue to ensure a valid initial state.
func insertQueue(t *testing.T, repo *QueueRepository, subscriptionID, messageID int64) *model.Queue {
	t.Helper()
	q := model.NewQueue(subscriptionID, messageID)
	saved, err := repo.Save(ctx(), &q)
	require.NoError(t, err)
	require.Greater(t, saved.ID, int64(0))
	return saved
}

func TestQueueRepository_SaveAndLoad(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	q := model.NewQueue(1, 10)
	saved, err := repo.Save(ctx(), &q)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0))
	assert.Equal(t, int64(1), saved.SubscriptionID)
	assert.Equal(t, int64(10), saved.MessageID)
	assert.Equal(t, model.QueueStatusPending, saved.Status)
	assert.Equal(t, 0, saved.AttemptCount)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, loaded.ID)
	assert.Equal(t, model.QueueStatusPending, loaded.Status)
}

func TestQueueRepository_Load_NotFound(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	_, err := repo.Load(ctx(), 9999)
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

func TestQueueRepository_Update(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	saved := insertQueue(t, repo, 1, 1)
	saved.MarkFailed(nil, 30*time.Second)

	updated, err := repo.Save(ctx(), saved)
	require.NoError(t, err)
	assert.Equal(t, model.QueueStatusFailed, updated.Status)
	assert.Equal(t, 1, updated.AttemptCount)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueStatusFailed, loaded.Status)
}

func TestQueueRepository_Delete(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	saved := insertQueue(t, repo, 1, 1)

	err := repo.Delete(ctx(), saved)
	require.NoError(t, err)

	_, err = repo.Load(ctx(), saved.ID)
	assert.True(t, pubsub.IsNoData(err), "deleted queue item must not be found")
}

func TestQueueRepository_FindByMessageID(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	saved := insertQueue(t, repo, 5, 100)

	found, err := repo.FindByMessageID(ctx(), 5, 100)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, found.ID)
}

func TestQueueRepository_FindByMessageID_NotFound(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	_, err := repo.FindByMessageID(ctx(), 1, 9999)
	assert.True(t, pubsub.IsNoData(err))
}

func TestQueueRepository_FindBySubscriptionID(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	insertQueue(t, repo, 7, 1)
	insertQueue(t, repo, 7, 2)
	insertQueue(t, repo, 8, 3) // Different subscription.

	results, err := repo.FindBySubscriptionID(ctx(), 7)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestQueueRepository_FindBySubscriptionID_Empty(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	_, err := repo.FindBySubscriptionID(ctx(), 9999)
	assert.True(t, pubsub.IsNoData(err))
}

func TestQueueRepository_FindPendingItems(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	// Two pending items with next_retry_at in the past (ready to deliver).
	insertQueue(t, repo, 1, 1)
	insertQueue(t, repo, 1, 2)

	// One failed item — should not appear in pending.
	failedQ := model.NewQueue(1, 3)
	savedFailed, err := repo.Save(ctx(), &failedQ)
	require.NoError(t, err)
	savedFailed.MarkFailed(nil, 30*time.Second)
	_, err = repo.Save(ctx(), savedFailed)
	require.NoError(t, err)

	results, err := repo.FindPendingItems(ctx(), 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, model.QueueStatusPending, r.Status)
	}
}

func TestQueueRepository_FindPendingItems_Empty(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	_, err := repo.FindPendingItems(ctx(), 10)
	assert.True(t, pubsub.IsNoData(err))
}

func TestQueueRepository_FindRetryableItems(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	// Insert a failed item with next_retry_at in the past.
	failedQ := model.NewQueue(1, 10)
	saved, err := repo.Save(ctx(), &failedQ)
	require.NoError(t, err)
	saved.Status = model.QueueStatusFailed
	saved.AttemptCount = 1
	// Set next_retry_at to 1 minute ago so it is immediately retryable.
	pastTime := time.Now().Add(-1 * time.Minute)
	saved.NextRetryAt = sql.NullTime{Time: pastTime, Valid: true}
	_, err = repo.Save(ctx(), saved)
	require.NoError(t, err)

	results, err := repo.FindRetryableItems(ctx(), 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, model.QueueStatusFailed, results[0].Status)
}

func TestQueueRepository_FindRetryableItems_Empty(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	_, err := repo.FindRetryableItems(ctx(), 10)
	assert.True(t, pubsub.IsNoData(err))
}

func TestQueueRepository_FindExpiredItems(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	// Insert a pending item and then set expires_at to the past via direct SQL.
	q := model.NewQueue(1, 1)
	saved, err := repo.Save(ctx(), &q)
	require.NoError(t, err)

	pastExpiry := time.Now().Add(-1 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	_, err = testDB.Exec("UPDATE pubsub_queue SET expires_at = ? WHERE id = ?", pastExpiry, saved.ID)
	require.NoError(t, err)

	// Insert a non-expired item.
	insertQueue(t, repo, 1, 2)

	results, err := repo.FindExpiredItems(ctx(), 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, saved.ID, results[0].ID)
}

func TestQueueRepository_FindExpiredItems_Empty(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	_, err := repo.FindExpiredItems(ctx(), 10)
	assert.True(t, pubsub.IsNoData(err))
}

func TestQueueRepository_UpdateNextRetry(t *testing.T) {
	truncate(t, "pubsub_queue")
	repo := NewQueueRepository(testDB, "sqlite3")

	saved := insertQueue(t, repo, 1, 1)

	nextRetry := time.Now().Add(5 * time.Minute).Truncate(time.Second)
	err := repo.UpdateNextRetry(ctx(), saved.ID, nextRetry, 1)
	require.NoError(t, err)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.AttemptCount)
	require.True(t, loaded.NextRetryAt.Valid)
	// Truncate to second for SQLite timestamp comparison.
	assert.Equal(t, nextRetry.UTC(), loaded.NextRetryAt.Time.UTC().Truncate(time.Second))
}

// ---------------------------------------------------------------------------
// DLQRepository tests
// ---------------------------------------------------------------------------

// newTestDLQ creates a valid DeadLetterQueue fixture for testing.
func newTestDLQ(subscriptionID, messageID int64) model.DeadLetterQueue {
	now := time.Now()
	return model.NewDeadLetterQueue(
		subscriptionID,
		messageID,
		0, // originalQueueID
		5, // attemptCount
		"connection refused",
		"Max retry attempts exceeded (5 >= 5)",
		now.Add(-10*time.Minute),
		now.Add(-1*time.Minute),
		`{"action":"test"}`,
		"https://hook.example.com/callback",
	)
}

func TestDLQRepository_SaveAndLoad(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	dlq := newTestDLQ(1, 10)
	saved, err := repo.Save(ctx(), dlq)
	require.NoError(t, err)
	assert.Greater(t, saved.ID, int64(0))
	assert.Equal(t, int64(1), saved.SubscriptionID)
	assert.Equal(t, int64(10), saved.MessageID)
	assert.Equal(t, "Max retry attempts exceeded (5 >= 5)", saved.FailureReason)
	assert.False(t, saved.IsResolved)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, loaded.ID)
	assert.Equal(t, int64(10), loaded.MessageID)
	assert.False(t, loaded.IsResolved)
}

func TestDLQRepository_Load_NotFound(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	_, err := repo.Load(ctx(), 9999)
	require.Error(t, err)
	assert.True(t, pubsub.IsNoData(err))
}

func TestDLQRepository_Update_Resolve(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	dlq := newTestDLQ(1, 1)
	saved, err := repo.Save(ctx(), dlq)
	require.NoError(t, err)

	saved.Resolve("admin", "Manually replayed and confirmed received")
	updated, err := repo.Save(ctx(), saved)
	require.NoError(t, err)
	assert.True(t, updated.IsResolved)
	assert.Equal(t, "admin", updated.ResolvedBy)

	loaded, err := repo.Load(ctx(), saved.ID)
	require.NoError(t, err)
	assert.True(t, loaded.IsResolved)
	assert.Equal(t, "admin", loaded.ResolvedBy)
}

func TestDLQRepository_Delete(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	dlq := newTestDLQ(1, 1)
	saved, err := repo.Save(ctx(), dlq)
	require.NoError(t, err)

	err = repo.Delete(ctx(), saved)
	require.NoError(t, err)

	_, err = repo.Load(ctx(), saved.ID)
	assert.True(t, pubsub.IsNoData(err), "deleted DLQ item must not be found")
}

func TestDLQRepository_FindBySubscription(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	saved1, err := repo.Save(ctx(), newTestDLQ(3, 1))
	require.NoError(t, err)
	saved2, err := repo.Save(ctx(), newTestDLQ(3, 2))
	require.NoError(t, err)
	_, err = repo.Save(ctx(), newTestDLQ(4, 3)) // Different subscription.
	require.NoError(t, err)

	results, err := repo.FindBySubscription(ctx(), 3, 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	ids := []int64{results[0].ID, results[1].ID}
	assert.Contains(t, ids, saved1.ID)
	assert.Contains(t, ids, saved2.ID)
}

func TestDLQRepository_FindBySubscription_Empty(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	_, err := repo.FindBySubscription(ctx(), 9999, 10)
	assert.True(t, pubsub.IsNoData(err))
}

func TestDLQRepository_FindUnresolved(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	// Two unresolved items.
	_, err := repo.Save(ctx(), newTestDLQ(1, 1))
	require.NoError(t, err)
	_, err = repo.Save(ctx(), newTestDLQ(1, 2))
	require.NoError(t, err)

	// One resolved item.
	resolved := newTestDLQ(1, 3)
	savedResolved, err := repo.Save(ctx(), resolved)
	require.NoError(t, err)
	savedResolved.Resolve("system", "auto-resolved")
	_, err = repo.Save(ctx(), savedResolved)
	require.NoError(t, err)

	results, err := repo.FindUnresolved(ctx(), 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.False(t, r.IsResolved)
	}
}

func TestDLQRepository_FindUnresolved_Empty(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	_, err := repo.FindUnresolved(ctx(), 10)
	assert.True(t, pubsub.IsNoData(err))
}

func TestDLQRepository_FindOlderThan(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	// Save an item and then backdate created_at via direct SQL.
	dlq := newTestDLQ(1, 1)
	saved, err := repo.Save(ctx(), dlq)
	require.NoError(t, err)

	oldTime := time.Now().Add(-3 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	_, err = testDB.Exec("UPDATE pubsub_dead_letter_queue SET created_at = ? WHERE id = ?", oldTime, saved.ID)
	require.NoError(t, err)

	// Insert a recent item.
	_, err = repo.Save(ctx(), newTestDLQ(1, 2))
	require.NoError(t, err)

	results, err := repo.FindOlderThan(ctx(), 2*time.Hour, 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, saved.ID, results[0].ID)
}

func TestDLQRepository_FindOlderThan_Empty(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	_, err := repo.FindOlderThan(ctx(), 24*time.Hour, 10)
	assert.True(t, pubsub.IsNoData(err))
}

func TestDLQRepository_FindByMessageID(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	dlq := newTestDLQ(1, 42)
	saved, err := repo.Save(ctx(), dlq)
	require.NoError(t, err)

	found, err := repo.FindByMessageID(ctx(), 42)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, found.ID)
	assert.Equal(t, int64(42), found.MessageID)
}

func TestDLQRepository_FindByMessageID_NotFound(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	_, err := repo.FindByMessageID(ctx(), 9999)
	assert.True(t, pubsub.IsNoData(err))
}

func TestDLQRepository_GetStats(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	// Two unresolved items.
	_, err := repo.Save(ctx(), newTestDLQ(1, 1))
	require.NoError(t, err)
	_, err = repo.Save(ctx(), newTestDLQ(1, 2))
	require.NoError(t, err)

	// One resolved item.
	resolved := newTestDLQ(1, 3)
	savedR, err := repo.Save(ctx(), resolved)
	require.NoError(t, err)
	savedR.Resolve("ops", "fixed")
	_, err = repo.Save(ctx(), savedR)
	require.NoError(t, err)

	stats, err := repo.GetStats(ctx())
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalItems)
	assert.Equal(t, 2, stats.UnresolvedItems)
	assert.Equal(t, 1, stats.ResolvedItems)
}

func TestDLQRepository_GetStats_Empty(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	stats, err := repo.GetStats(ctx())
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalItems)
	assert.Equal(t, 0, stats.UnresolvedItems)
}

func TestDLQRepository_CountUnresolved(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	_, err := repo.Save(ctx(), newTestDLQ(1, 1))
	require.NoError(t, err)
	_, err = repo.Save(ctx(), newTestDLQ(1, 2))
	require.NoError(t, err)

	resolved := newTestDLQ(1, 3)
	savedR, err := repo.Save(ctx(), resolved)
	require.NoError(t, err)
	savedR.Resolve("ops", "done")
	_, err = repo.Save(ctx(), savedR)
	require.NoError(t, err)

	count, err := repo.CountUnresolved(ctx())
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestDLQRepository_CountUnresolved_Zero(t *testing.T) {
	truncate(t, "pubsub_dead_letter_queue")
	repo := NewDLQRepository(testDB, "sqlite3")

	count, err := repo.CountUnresolved(ctx())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// ---------------------------------------------------------------------------
// NewRepositories factory tests
// ---------------------------------------------------------------------------

func TestNewRepositories(t *testing.T) {
	repos := NewRepositories(testDB, "sqlite3")

	assert.NotNil(t, repos.Queue)
	assert.NotNil(t, repos.Message)
	assert.NotNil(t, repos.Subscription)
	assert.NotNil(t, repos.DLQ)
	assert.NotNil(t, repos.Publisher)
	assert.NotNil(t, repos.Subscriber)
	assert.NotNil(t, repos.Topic)
}

func TestNewRepositoriesWithPrefix(t *testing.T) {
	repos := NewRepositoriesWithPrefix(testDB, "sqlite3", "custom_")

	assert.NotNil(t, repos.Queue)
	assert.NotNil(t, repos.Message)
	assert.NotNil(t, repos.Subscription)
	assert.NotNil(t, repos.DLQ)
	assert.NotNil(t, repos.Publisher)
	assert.NotNil(t, repos.Subscriber)
	assert.NotNil(t, repos.Topic)
}
