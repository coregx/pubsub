# PubSub

**Production-ready Pub/Sub library and standalone service for Go**

Works both as a **library** for embedding in your application AND as a **standalone microservice** with REST API.

[![CI](https://github.com/coregx/pubsub/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/coregx/pubsub/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/coregx/pubsub.svg)](https://pkg.go.dev/github.com/coregx/pubsub)
[![Go Report Card](https://goreportcard.com/badge/github.com/coregx/pubsub)](https://goreportcard.com/report/github.com/coregx/pubsub)
[![License](https://img.shields.io/github/license/coregx/pubsub)](LICENSE)
[![Release](https://img.shields.io/github/v/release/coregx/pubsub?include_prereleases)](https://github.com/coregx/pubsub/releases)

## ✨ Features

### Core Features
- **📨 Reliable Message Delivery** - Guaranteed delivery with exponential backoff retry
- **🔄 Exponential Backoff** - 30s → 1m → 2m → 4m → 8m → 16m → 30m (max)
- **💀 Dead Letter Queue (DLQ)** - Automatic handling of failed messages after 5 attempts
- **📊 DLQ Statistics** - Track failure reasons and resolution metrics
- **🎯 Domain-Driven Design** - Rich domain models with business logic
- **🗄️ Repository Pattern** - Clean data access abstraction

### Architecture
- **🔌 Pluggable** - Bring your own Logger, Notification system
- **⚙️ Options Pattern** - Modern Go API (2025 best practices)
- **🏗️ Clean Architecture** - Services, Repositories, Models separation
- **✅ Battle-Tested** - Production-proven in FreiCON Railway Management System

### Database Support
- **🐬 MySQL** - Full support with Relica adapters
- **🐘 PostgreSQL** - Full support with Relica adapters
- **🪶 SQLite** - Full support with Relica adapters
- **⚡ Zero Dependencies** - Relica query builder (no ORM bloat)

### Deployment Options
- **📚 As Library** - Embed in your Go application
- **🐳 As Service** - Standalone PubSub server with REST API
- **☸️ Docker Ready** - Production Dockerfile + docker-compose
- **🌐 Cloud Native** - 12-factor app, ENV config, health checks

## 📦 Installation

### As Library
```bash
go get github.com/coregx/pubsub@latest
```

### As Standalone Service
```bash
# Using Docker (recommended)
cd cmd/pubsub-server
docker-compose up -d

# Or build from source
go build ./cmd/pubsub-server
```

## 🚦 Quick Start

### Option 1: Standalone Service (Fastest!)

```bash
# Windows
cd cmd/pubsub-server
start.bat

# Linux/Mac
cd cmd/pubsub-server
docker-compose up -d
```

Access API at `http://localhost:8080`

See [Server Documentation](cmd/pubsub-server/README.md) for API endpoints.

### Option 2: Embedded Library

```go
package main

import (
    "context"
    "database/sql"
    "time"

    "github.com/coregx/pubsub"
    "github.com/coregx/pubsub/adapters/relica"
    _ "github.com/go-sql-driver/mysql"
)

func main() {
    // Connect to database
    db, _ := sql.Open("mysql", "user:pass@tcp(localhost:3306)/pubsub?parseTime=true")

    // Create repositories (production-ready Relica adapters!)
    repos := relica.NewRepositories(db, "mysql")

    // Create services
    publisher, _ := pubsub.NewPublisher(
        pubsub.WithPublisherRepositories(
            repos.Message, repos.Queue, repos.Subscription, repos.Topic,
        ),
        pubsub.WithPublisherLogger(logger),
    )

    // Publish message
    result, _ := publisher.Publish(context.Background(), pubsub.PublishRequest{
        TopicCode:  "user.signup",
        Identifier: "user-123",
        Data:       `{"userId": 123, "email": "user@example.com"}`,
    })

    // Create worker for background processing
    worker, _ := pubsub.NewQueueWorker(
        pubsub.WithRepositories(repos.Queue, repos.Message, repos.Subscription, repos.DLQ),
        pubsub.WithDelivery(transmitterProvider, gateway),
        pubsub.WithLogger(logger),
    )

    // Run worker (processes queue every 30 seconds)
    worker.Run(context.Background(), 30*time.Second)
}
```

## 🗄️ Database Setup

### Using Embedded Migrations (Recommended)

```go
import "github.com/coregx/pubsub/migrations"

// Apply all migrations
if err := migrations.ApplyAll(db); err != nil {
    log.Fatal(err)
}
```

### Manual Migrations

```bash
# MySQL
mysql -u user -p database < migrations/mysql/001_core_tables.sql
mysql -u user -p database < migrations/mysql/002_retry_fields.sql
mysql -u user -p database < migrations/mysql/003_dead_letter_queue.sql

# PostgreSQL
psql -U user -d database -f migrations/postgres/001_core_tables.sql
...

# SQLite
sqlite3 pubsub.db < migrations/sqlite/001_core_tables.sql
...
```

## 🏗️ Architecture

```
┌─────────────────────────────────────┐
│         Your Application            │
│  (or REST API for standalone)       │
└─────────────┬───────────────────────┘
              │
┌─────────────▼───────────────────────┐
│          Services Layer             │
│  - Publisher                        │
│  - SubscriptionManager              │
│  - QueueWorker                      │
└─────────────┬───────────────────────┘
              │
┌─────────────▼───────────────────────┐
│       Relica Adapters               │
│  (Production-Ready Implementations) │
│  - Zero dependencies                │
│  - MySQL / PostgreSQL / SQLite      │
└─────────────┬───────────────────────┘
              │
┌─────────────▼───────────────────────┐
│          Database                   │
└─────────────────────────────────────┘
```

## 📡 REST API (Standalone Service)

When running as standalone service, PubSub-Go exposes these endpoints:

### Publish Message
```bash
POST /api/v1/publish
Content-Type: application/json

{
  "topicCode": "user.signup",
  "identifier": "optional-dedup-key",
  "data": {
    "userId": 123,
    "email": "user@example.com"
  }
}
```

### Subscribe to Topic
```bash
POST /api/v1/subscribe
{
  "subscriberId": 1,
  "topicCode": "user.signup",
  "identifier": "webhook-receiver-1"
}
```

### List Subscriptions
```bash
GET /api/v1/subscriptions?subscriberId=1
```

### Unsubscribe
```bash
DELETE /api/v1/subscriptions/123
```

### Health Check
```bash
GET /api/v1/health
```

See [API Documentation](cmd/pubsub-server/README.md) for full details.

## 🔧 Configuration

### Library Configuration (Go)

```go
// Options Pattern (2025 best practice)
worker, err := pubsub.NewQueueWorker(
    pubsub.WithRepositories(queueRepo, msgRepo, subRepo, dlqRepo),
    pubsub.WithDelivery(transmitterProvider, gateway),
    pubsub.WithLogger(logger),
    pubsub.WithBatchSize(100),              // optional
    pubsub.WithRetryStrategy(customStrategy), // optional
    pubsub.WithNotifications(notifService),  // optional
)
```

### Service Configuration (ENV)

```bash
# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Database
DB_DRIVER=mysql
DB_HOST=localhost
DB_PORT=3306
DB_USER=pubsub
DB_PASSWORD=your_password
DB_NAME=pubsub
DB_PREFIX=pubsub_

# Worker
PUBSUB_BATCH_SIZE=100
PUBSUB_WORKER_INTERVAL=30
PUBSUB_ENABLE_NOTIFICATIONS=true
```

See [`.env.example`](cmd/pubsub-server/.env.example) for all options.

## 📊 How It Works

### Message Flow

```
1. PUBLISH
   Publisher → Topic → Create Message
                    → Find Active Subscriptions
                    → Create Queue Items (one per subscription)

2. WORKER (Background)
   QueueWorker → Find Pending/Retryable Items (batch)
              → Deliver to Subscribers (via webhooks/gateway)
              → On Success: Mark as SENT
              → On Failure: Retry with exponential backoff
              → After 5 failures: Move to DLQ

3. DLQ (Dead Letter Queue)
   Failed items → Manual review
               → Resolve or Delete
```

### Retry Schedule

```
Attempt 1: Immediate
Attempt 2: +30 seconds
Attempt 3: +1 minute
Attempt 4: +2 minutes
Attempt 5: +4 minutes
Attempt 6: +8 minutes (moves to DLQ after this)
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# With coverage
go test ./... -cover

# Model tests (95.9% coverage)
go test ./model/... -cover

# Integration tests (requires database)
go test ./adapters/relica/... -cover
```

## 🐳 Docker Deployment

### Quick Start
```bash
cd cmd/pubsub-server
docker-compose up -d
```

### Production Build
```bash
# Build image
docker build -t pubsub-server:0.1.0 -f cmd/pubsub-server/Dockerfile .

# Run with environment
docker run -d \
  -p 8080:8080 \
  -e DB_DRIVER=mysql \
  -e DB_HOST=mysql \
  -e DB_PASSWORD=secret \
  pubsub-server:0.1.0
```

## 📚 Examples

- [Basic Example](examples/basic/main.go) - Simple QueueWorker setup with Relica
- [Server Example](cmd/pubsub-server/main.go) - Full standalone service

## 🗺️ Roadmap

### v0.1.0 (Current - Alpha) ✅
- [x] Core PubSub functionality
- [x] Relica adapters (MySQL/PostgreSQL/SQLite)
- [x] Publisher + SubscriptionManager services
- [x] Standalone REST API server
- [x] Docker support
- [x] Health checks

### v0.2.0 (Next)
- [ ] Delivery providers (HTTP webhooks, gRPC)
- [ ] Message encryption
- [ ] Rate limiting
- [ ] Metrics (Prometheus)
- [ ] Admin UI

### v1.0.0 (Stable)
- [ ] OpenAPI/Swagger docs
- [ ] Authentication/Authorization
- [ ] Multi-tenancy
- [ ] Message replay
- [ ] Full test coverage (>90%)

## 🤝 Contributing

This is an alpha release. Contributions welcome!

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing`)
3. Commit changes (`git commit -m 'feat: add amazing feature'`)
4. Push to branch (`git push origin feature/amazing`)
5. Open Pull Request

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Relica** - Type-safe query builder (github.com/coregx/relica)
- **FreiCON** - Original production testing ground
- **CoreGX Ecosystem** - Part of CoreGX microservices suite

## 📞 Support

- 🐛 **Issues**: [GitHub Issues](https://github.com/coregx/pubsub/issues)
- 📖 **Documentation**: [Wiki](https://github.com/coregx/pubsub/wiki)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/coregx/pubsub/discussions)

---

**⚠️ Pre-Release Status**

This is a pre-release version (v0.1.0 development). The library is production-ready and battle-tested in FreiCON Railway Management System with 95.9% test coverage and zero linter issues. APIs may evolve before v1.0.0 LTS release.

**📦 Dependencies**

This library uses [Relica](https://github.com/coregx/relica) for type-safe database operations. All dependencies are properly published and available through Go modules.

---

Made with ❤️ by CoreGX Team
