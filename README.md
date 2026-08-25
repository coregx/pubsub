# PubSub

**Production-ready Pub/Sub library and standalone service for Go**

Works both as a **library** for embedding in your application AND as a **standalone microservice** with REST API.

[![CI](https://github.com/coregx/pubsub/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/coregx/pubsub/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/coregx/pubsub.svg)](https://pkg.go.dev/github.com/coregx/pubsub)
[![Go Report Card](https://goreportcard.com/badge/github.com/coregx/pubsub)](https://goreportcard.com/report/github.com/coregx/pubsub)
[![codecov](https://codecov.io/gh/coregx/pubsub/branch/main/graph/badge.svg)](https://codecov.io/gh/coregx/pubsub)
[![License](https://img.shields.io/github/license/coregx/pubsub)](LICENSE)
[![Release](https://img.shields.io/github/v/release/coregx/pubsub)](https://github.com/coregx/pubsub/releases)

## Features

### Core
- **Reliable Message Delivery** — Guaranteed delivery with exponential backoff retry
- **Exponential Backoff** — 30s → 1m → 2m → 4m → 8m → 16m → 30m (max)
- **Dead Letter Queue (DLQ)** — Automatic handling of failed messages after 5 attempts
- **DLQ Statistics** — Track failure reasons and resolution metrics
- **Domain-Driven Design** — Rich domain models with business logic
- **Repository Pattern** — Clean data access abstraction

### Architecture
- **Pluggable** — Bring your own Logger, Notification system, Delivery gateway
- **Options Pattern** — Modern Go API (2026 best practices)
- **Clean Architecture** — Services, Repositories, Models separation
- **Battle-Tested** — Production-proven in FreiCON Railway Management System

### Database Support
- **MySQL** — Full support with [Relica](https://github.com/coregx/relica) adapters
- **PostgreSQL** — Full support with Relica adapters
- **SQLite** — Full support with Relica adapters
- **Type-Safe Queries** — Relica v0.14 expression API (no raw SQL)

### Deployment Options
- **As Library** — Embed in your Go application
- **As Service** — Standalone PubSub server with REST API ([Fursy](https://github.com/coregx/fursy) framework)
- **Docker Ready** — Production Dockerfile + docker-compose
- **Cloud Native** — 12-factor app, ENV config, health checks

## Installation

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

## Quick Start

### Option 1: Standalone Service

```bash
cd cmd/pubsub-server
docker-compose up -d
```

Access API at `http://localhost:8080`

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
    db, _ := sql.Open("mysql", "user:pass@tcp(localhost:3306)/pubsub?parseTime=true")

    repos := relica.NewRepositories(db, "mysql")

    publisher, _ := pubsub.NewPublisher(
        pubsub.WithPublisherRepositories(
            repos.Message, repos.Queue, repos.Subscription, repos.Topic,
        ),
        pubsub.WithPublisherLogger(logger),
    )

    result, _ := publisher.Publish(context.Background(), pubsub.PublishRequest{
        TopicCode:  "user.signup",
        Identifier: "user-123",
        Data:       `{"userId": 123, "email": "user@example.com"}`,
    })

    worker, _ := pubsub.NewQueueWorker(
        pubsub.WithRepositories(repos.Queue, repos.Message, repos.Subscription, repos.DLQ),
        pubsub.WithDelivery(transmitterProvider, gateway),
        pubsub.WithLogger(logger),
    )

    worker.Run(context.Background(), 30*time.Second)
}
```

## Architecture

```
┌─────────────────────────────────────┐
│         Your Application            │
│  (or Fursy REST API for standalone)  │
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
│    Relica v0.14 Adapters            │
│  Type-safe expression API           │
│  MySQL / PostgreSQL / SQLite        │
└─────────────┬───────────────────────┘
              │
┌─────────────▼───────────────────────┐
│          Database                   │
└─────────────────────────────────────┘
```

## REST API (Standalone Service)

The standalone server uses [Fursy](https://github.com/coregx/fursy) framework with type-safe generic handlers and RFC 9457 Problem Details for error responses.

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
  "subscriberID": 1,
  "topicCode": "user.signup",
  "identifier": "webhook-receiver-1"
}
```

### List Subscriptions
```bash
GET /api/v1/subscriptions?subscriberID=1
```

### Unsubscribe
```bash
DELETE /api/v1/subscriptions/123
```

### Health Check
```bash
GET /api/v1/health
```

## Configuration

### Library Configuration (Go)

```go
worker, err := pubsub.NewQueueWorker(
    pubsub.WithRepositories(queueRepo, msgRepo, subRepo, dlqRepo),
    pubsub.WithDelivery(transmitterProvider, gateway),
    pubsub.WithLogger(logger),
    pubsub.WithBatchSize(100),
    pubsub.WithRetryStrategy(customStrategy),
    pubsub.WithNotifications(notifService),
)
```

### Service Configuration (ENV)

```bash
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

DB_DRIVER=mysql          # mysql, postgres, sqlite3
DB_HOST=localhost
DB_PORT=3306
DB_USER=pubsub
DB_PASSWORD=your_password
DB_NAME=pubsub
DB_PREFIX=pubsub_

PUBSUB_BATCH_SIZE=100
PUBSUB_WORKER_INTERVAL=30
PUBSUB_ENABLE_NOTIFICATIONS=true
```

## How It Works

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

## Testing

```bash
# Run unit tests
go test ./model/... ./retry/... . ./cmd/pubsub-server/internal/config/...

# Run with coverage
go test -cover ./model/... ./retry/... . ./cmd/pubsub-server/internal/api/...

# Integration tests (requires CGO for SQLite)
CGO_ENABLED=1 go test ./adapters/relica/...

# All tests
go test ./...
```

### Test Coverage

| Package | Coverage |
|---------|----------|
| `model/` | 95.9% |
| `retry/` | 100% |
| `pubsub` (root) | 82.4% |
| `config/` | 100% |
| `api/` (handlers) | 94.4% |
| `adapters/relica/` | CI only (SQLite CGO) |

## Docker Deployment

```bash
# Build image
docker build -t pubsub-server:0.2.0 -f cmd/pubsub-server/Dockerfile .

# Run with environment
docker run -d \
  -p 8080:8080 \
  -e DB_DRIVER=mysql \
  -e DB_HOST=mysql \
  -e DB_PASSWORD=secret \
  pubsub-server:0.2.0
```

## Roadmap

### v0.1.0 (Released)
- [x] Core PubSub functionality
- [x] Relica adapters (MySQL/PostgreSQL/SQLite)
- [x] Publisher + SubscriptionManager services
- [x] Standalone REST API server
- [x] Docker support

### v0.2.0 (Current)
- [x] Relica v0.14 type-safe expression API
- [x] Fursy v0.4 HTTP framework with generic handlers
- [x] RFC 9457 Problem Details for error responses
- [x] Comprehensive test suite (82-100% coverage)
- [x] CI with MySQL/PostgreSQL service containers
- [x] OIDC Codecov integration
- [ ] Delivery providers (HTTP webhooks, gRPC)
- [ ] Message encryption
- [ ] Rate limiting
- [ ] Prometheus metrics

### v1.0.0 (Planned)
- [ ] API stability guarantee
- [ ] Long-term support (3+ years)
- [ ] OpenAPI/Swagger docs
- [ ] Authentication/Authorization

## Contributing

Contributions welcome!

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing`)
3. Commit changes (`git commit -m 'feat: add amazing feature'`)
4. Push to branch (`git push origin feature/amazing`)
5. Open Pull Request

See [CONTRIBUTING.md](CONTRIBUTING.md) for full guidelines.

## License

MIT License — see [LICENSE](LICENSE) file for details.

## Dependencies

- **[Relica](https://github.com/coregx/relica)** v0.14 — Type-safe database query builder
- **[Fursy](https://github.com/coregx/fursy)** v0.4 — HTTP framework (standalone server only)
- **[ozzo-validation](https://github.com/go-ozzo/ozzo-validation)** — Request validation

## Support

- **Issues**: [GitHub Issues](https://github.com/coregx/pubsub/issues)
- **Discussions**: [GitHub Discussions](https://github.com/coregx/pubsub/discussions)

---

Production-ready and battle-tested in FreiCON Railway Management System. Part of the [CoreGX](https://github.com/coregx) ecosystem.


## Star History

<a href="https://starhistory.io">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.starhistory.io/png?repos=coregx/pubsub&style=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.starhistory.io/png?repos=coregx/pubsub&style=professional" />
   <img alt="Star History Chart" src="https://api.starhistory.io/png?repos=coregx/pubsub" width="800" />
 </picture>
</a>
