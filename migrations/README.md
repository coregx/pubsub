# Database Migrations for PubSub-Go

## Table Prefix Configuration

PubSub-Go uses table prefixes to allow flexible database deployment:

- **Default prefix**: `pubsub_`
- **Custom prefix**: Set in `model/prefix.go`

### Supported Deployment Scenarios

#### 1. Dedicated Database (Recommended for Production)
```sql
CREATE DATABASE pubsub_production;
USE pubsub_production;
-- Run migrations with default prefix
```

#### 2. Shared Database with Prefixes (Like FreiCON)
```sql
USE your_application_db;
-- Tables will be: pubsub_queue, pubsub_message, etc.
-- Run migrations with default or custom prefix
```

#### 3. Custom Prefix for Multi-Tenant
```go
// In your project, change model/prefix.go:
const tablePrefix = "tenant1_pubsub_"
```

## Migration Files

### 1. Core Tables (`001_core_tables.sql`)
Creates main PubSub tables:
- `{prefix}topic` - Topics for pub/sub
- `{prefix}publisher` - Publishers
- `{prefix}subscriber` - Subscribers
- `{prefix}subscription` - Subscriptions
- `{prefix}message` - Published messages
- `{prefix}queue` - Delivery queue

### 2. Retry Fields (`002_retry_fields.sql`)
Adds retry logic fields to queue table:
- `attempt_count`, `last_attempt_at`
- `next_retry_at`, `last_error`
- Exponential backoff support

### 3. Dead Letter Queue (`003_dead_letter_queue.sql`)
Creates Dead Letter Queue table:
- `{prefix}dlq` - Failed messages after max retries
- Tracking of failure reasons

## How to Apply Migrations

### Option 1: Embedded Migrations (Recommended - 2025 Best Practice)

PubSub-Go embeds all migration files in the binary using Go's `embed` package. You can access them programmatically with your preferred migration tool.

#### Using Goose (Programmatic)
```go
import (
    "database/sql"
    "github.com/pressly/goose/v3"
    pubsub "github.com/freicon/pubsub-go"
)

db, err := sql.Open("mysql", "user:pass@tcp(host:3306)/dbname?parseTime=true")
if err != nil {
    log.Fatal(err)
}

// Set embedded filesystem as base
goose.SetBaseFS(pubsub.MigrationFiles)

// Apply all migrations
if err := goose.Up(db, "migrations"); err != nil {
    log.Fatal(err)
}

// Or apply specific version
if err := goose.UpTo(db, "migrations", 2); err != nil {
    log.Fatal(err)
}
```

#### Using golang-migrate
```go
import (
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/mysql"
    "github.com/golang-migrate/migrate/v4/source/iofs"
    pubsub "github.com/freicon/pubsub-go"
)

// Create source from embedded files
source, err := iofs.New(pubsub.MigrationFiles, "migrations")
if err != nil {
    log.Fatal(err)
}

// Create migrator
m, err := migrate.NewWithSourceInstance(
    "iofs",
    source,
    "mysql://user:pass@tcp(host:3306)/dbname?parseTime=true",
)
if err != nil {
    log.Fatal(err)
}

// Apply migrations
if err := m.Up(); err != nil && err != migrate.ErrNoChange {
    log.Fatal(err)
}
```

#### Using Atlas (Declarative)
```go
import (
    "context"
    "os"
    "path/filepath"
    "ariga.io/atlas-go-sdk/atlasexec"
    pubsub "github.com/freicon/pubsub-go"
)

// Extract migrations to temp directory
tmpDir, _ := os.MkdirTemp("", "migrations")
defer os.RemoveAll(tmpDir)

entries, _ := pubsub.MigrationFiles.ReadDir("migrations")
for _, entry := range entries {
    data, _ := pubsub.MigrationFiles.ReadFile("migrations/" + entry.Name())
    os.WriteFile(filepath.Join(tmpDir, entry.Name()), data, 0644)
}

// Apply with Atlas
client, _ := atlasexec.NewClient(".", "atlas")
_, err := client.MigrateApply(context.Background(), &atlasexec.MigrateApplyParams{
    URL:    "mysql://user:pass@tcp(host:3306)/dbname",
    DirURL: "file://" + tmpDir,
})
```

### Option 2: Manual SQL Execution
```bash
# With default prefix (pubsub_)
mysql -u user -p database < migrations/001_core_tables.sql
mysql -u user -p database < migrations/002_retry_fields.sql
mysql -u user -p database < migrations/003_dead_letter_queue.sql
```

### Option 3: Goose CLI
```bash
# Clone repository and run from migrations directory
goose -dir migrations mysql "user:pass@/dbname" up
```

### Option 4: Custom Prefix (Find & Replace)
```bash
# For custom prefix, replace in SQL files before applying
sed 's/pubsub_/myapp_pubsub_/g' 001_core_tables.sql > 001_custom.sql
mysql -u user -p database < 001_custom.sql
```

## Table Structure Summary

| Table | Purpose | Key Fields |
|-------|---------|-----------|
| `{prefix}topic` | Topics | id, code, name |
| `{prefix}publisher` | Publishers | id, code, name |
| `{prefix}subscriber` | Subscribers | id, name, client_id |
| `{prefix}subscription` | Subscriptions | id, subscriber_id, topic_id |
| `{prefix}message` | Messages | id, topic_id, publisher_id, payload |
| `{prefix}queue` | Delivery Queue | id, subscription_id, message_id, status, attempt_count |
| `{prefix}dlq` | Dead Letter Queue | id, queue_id, reason, moved_at |

## Indexes

All tables have optimized indexes for:
- Primary lookups (id)
- Foreign key relations
- Query performance (status, dates)
- Retry scheduling (next_retry_at)

## Best Practices

### For Production
1. ✅ Use dedicated database
2. ✅ Keep default `pubsub_` prefix
3. ✅ Apply migrations via migration tool (Goose, golang-migrate, etc.)
4. ✅ Use embedded migrations for programmatic control
5. ✅ Backup before migration
6. ❌ Don't use auto-migration

### For Development
1. ✅ Shared database with prefix is OK
2. ✅ Can use embedded migrations for quick setup
3. ✅ Reset database frequently

### For Multi-Tenant
1. ✅ Use custom prefix per tenant: `tenant_{id}_pubsub_`
2. ✅ Or separate database per tenant
3. ✅ Document prefix strategy in your project

## Migration History

| Version | File | Description |
|---------|------|-------------|
| 1.0 | 001_core_tables.sql | Initial PubSub tables |
| 1.1 | 002_retry_fields.sql | Retry logic with exponential backoff |
| 1.2 | 003_dead_letter_queue.sql | Dead Letter Queue for failed messages |

## Rollback

```sql
-- To rollback, drop tables in reverse order:
DROP TABLE IF EXISTS {prefix}dlq;
DROP TABLE IF EXISTS {prefix}queue;
DROP TABLE IF EXISTS {prefix}message;
DROP TABLE IF EXISTS {prefix}subscription;
DROP TABLE IF EXISTS {prefix}subscriber;
DROP TABLE IF EXISTS {prefix}publisher;
DROP TABLE IF EXISTS {prefix}topic;
```

## Questions?

- For migration issues: [GitHub Issues](https://github.com/freicon/pubsub-go/issues)
- For prefix customization: See `model/prefix.go`
- For examples: See `/examples` directory
