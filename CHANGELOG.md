# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Upcoming Features
- HTTP webhook delivery provider
- gRPC delivery provider
- Message encryption
- Rate limiting
- Prometheus metrics

---

## [0.2.0] - 2026-07-18

### Relica v0.14.1 Migration

The entire data access layer has been modernized to use Relica v0.14.1 recommended patterns:

- **Type-safe expression API** — All queries use `relica.Eq()`, `relica.And()`, `relica.LessOrEqual()`, `relica.NotEq()`, `relica.LessThan()` instead of positional placeholders
- **`relica.ErrNotFound`** — Replaced `sql.ErrNoRows` checks with Relica sentinel error
- **`Count()`** — DLQ statistics use Relica convenience method instead of manual `SELECT COUNT(*)`
- **`AndWhere()`** — Dynamic conditional WHERE clauses use explicit `AndWhere()` method
- **Cross-DB compatibility** — Removed MySQL-specific `DATE_SUB(NOW(), INTERVAL ? DAY)`, replaced with Go `time.Now().AddDate()`
- **`defaultTablePrefix`** — Extracted repeated `"pubsub_"` string into package constant

### Fursy v0.4.0 HTTP Framework

The standalone server has been migrated from `net/http` to [Fursy](https://github.com/coregx/fursy):

- **Type-safe generic handlers** — `fursy.Box[Req, Res]` with automatic JSON binding
- **RFC 9457 Problem Details** — Standardized error responses via `box.Problem()`
- **Typed response structs** — Replaced `map[string]interface{}` with `HealthResponse`, `PublishResponse`, etc.
- **Built-in middleware** — `middleware.Logger()` and `middleware.Recovery()`
- **Graceful shutdown** — `router.ListenAndServeWithShutdown()` with `OnShutdown` callbacks
- **Path parameters** — `box.Param("id")` instead of manual `splitPath()` parsing
- **`json.RawMessage`** — Zero-copy passthrough for publish data

### Comprehensive Test Suite

- **75 unit tests** for Publisher, QueueWorker, SubscriptionManager (82.4% coverage)
- **22 HTTP handler tests** with httptest + Fursy router (94.4% coverage)
- **12 config tests** for environment loading and validation (100% coverage)
- **Integration tests** for all 7 Relica adapters with SQLite in-memory
- **0 linter issues** across entire codebase

### CI/CD Improvements

- **OIDC Codecov** — Replaced token-based authentication with GitHub OIDC
- **Integration test job** — SQLite adapter tests on Linux with CGO
- **MySQL 8.0 service container** — Integration tests against real MySQL
- **PostgreSQL 16 service container** — Integration tests against real PostgreSQL
- **Separate unit/integration tests** — CGO-free unit tests on all 3 OS

### Dependencies Updated

| Dependency | From | To |
|-----------|------|-----|
| `coregx/relica` | v0.7.0 | v0.14.1 |
| `coregx/fursy` | — | v0.4.0 (new) |
| `go-sql-driver/mysql` | v1.9.3 | v1.10.0 |
| `lib/pq` | v1.10.9 | v1.12.3 |
| `mattn/go-sqlite3` | v1.14.32 | v1.14.48 |
| `filippo.io/edwards25519` | v1.1.0 | v1.2.0 |
| `opentelemetry/otel` | v1.21.0 | removed (unused) |

### Quality Metrics

| Metric | v0.1.0 | v0.2.0 |
|--------|--------|--------|
| Test coverage (model) | 95.9% | 95.9% |
| Test coverage (retry) | 100% | 100% |
| Test coverage (root) | 0% | 82.4% |
| Test coverage (config) | 0% | 100% |
| Test coverage (handlers) | 0% | 94.4% |
| Linter issues | 0 | 0 |
| CI databases | — | SQLite + MySQL + PostgreSQL |

---

## [0.1.0] - 2025-11-24

### 🎯 Initial Public Release

**Status**: Production-ready
**Focus**: Reliable pub/sub library and standalone service for Go 1.25+
**Quality**: 95.9% test coverage, 0 linter issues, clean architecture

### ✨ Features Implemented

#### Core Pub/Sub Implementation
- **Message Publishing** - Publisher service with topic-based routing
- **Queue Worker** - Background worker with exponential backoff retry
- **Subscription Management** - Full CRUD operations for subscriptions
- **Dead Letter Queue (DLQ)** - Failed message tracking with statistics
- **Retry Strategy** - Configurable exponential backoff (30s → 30m)
  - Default: 30s → 1m → 2m → 4m → 8m → 16m → 30m
  - DLQ after 5 failed attempts
  - Custom strategies via Options Pattern

#### Domain-Driven Design
- **Rich Domain Models** - Business logic in models (Queue, Message, Subscription, etc.)
- **Repository Pattern** - Clean data access abstraction via interfaces
- **Relica Adapters** - Production-ready implementations for MySQL, PostgreSQL, SQLite
- **Zero Dependencies** - Relica query builder (no ORM bloat)

#### Standalone Service
- **REST API Server** - Full HTTP API in `cmd/pubsub-server/`
- **Docker Support** - Production Dockerfile with multi-stage builds
- **Health Checks** - `/api/v1/health` endpoint
- **Environment Config** - 12-factor app compliance

#### Database Support
- **Multi-Database** - MySQL, PostgreSQL, SQLite via [Relica](https://github.com/coregx/relica)
- **Embedded Migrations** - Automatic schema setup
- **Migration Files** - SQL migrations embedded in binary
- **Relica Integration** - Type-safe struct operations with auto-populate ID

#### Quality & Documentation
- **Professional Godoc** - Complete API documentation for all exported symbols
- **High Test Coverage** - 95.9% coverage across all packages
- **Clean Architecture** - Application → Domain → Infrastructure → Database
- **Options Pattern** - Modern Go API design (2025 best practices)
- **Battle-Tested** - Used in FreiCON Railway Management System

### 📊 Quality Metrics

- **Test Coverage**: 95.9% (target: >90%)
- **Linter Issues**: 0 (golangci-lint with 34+ linters)
- **TODO Comments**: 0 (production-ready codebase)
- **Cross-Platform**: Linux, macOS, Windows
- **Go Version**: 1.25+

### 📚 Documentation

- **Complete Godoc** - All exported symbols documented
- **Code of Conduct** - Contributor Covenant v2.1
- **Contributing Guide** - Full Git-Flow workflow
- **Architecture Docs** - Clean Architecture + DDD patterns
- **Examples** - Working examples in `examples/`

### 🔧 Configuration Files

- **GitHub Actions** - Automated testing on main + develop branches
- **Codecov** - Coverage monitoring (90% target)
- **golangci-lint** - Aggressive configuration for code quality
- **CODEOWNERS** - Automatic code review assignments

### 📝 Files Created

#### Core Library
- `publisher.go` - Message publishing service
- `queue_worker.go` - Background delivery worker
- `subscription_manager.go` - Subscription lifecycle
- `repositories.go` - Repository interfaces (7 repositories)
- `options.go` - Options Pattern implementation
- `logger.go` - Pluggable logger interface
- `errors.go` - Custom error types
- `io.go` - I/O utilities
- `migrations.go` - Embedded migration support

#### Domain Models (model/)
- `queue.go` - Queue item with retry logic (10+ business methods)
- `message.go` - Published message
- `subscription.go` - Subscription mapping
- `dead_letter_queue.go` - DLQ with resolution tracking
- `publisher.go` - Publisher configuration
- `subscriber.go` - Subscriber with webhook URL
- `topic.go` - Topic/channel definition
- `data.go` - Message delivery format

#### Infrastructure (adapters/relica/)
- Message, Queue, Subscription, DLQ repositories
- Publisher, Subscriber, Topic repositories
- Factory functions for all repos
- [Relica](https://github.com/coregx/relica) Model() API integration with auto-populate ID

#### Retry Strategy (retry/)
- `middleware.go` - Exponential backoff strategy
- Configurable delays, max attempts, DLQ threshold
- Production-tested defaults

#### Standalone Server (cmd/pubsub-server/)
- REST API with routing
- Configuration management
- Health check endpoints
- Docker support

#### Documentation
- `README.md` - Project overview
- `LICENSE` - MIT License
- `CODE_OF_CONDUCT.md` - Contributor Covenant
- `CONTRIBUTING.md` - Development workflow
- `CHANGELOG.md` - This file
- `SECURITY.md` - Security policy

#### Configuration
- `.golangci.yml` - Linter configuration
- `.codecov.yml` - Coverage monitoring
- `.github/workflows/test.yml` - CI/CD pipeline
- `.github/CODEOWNERS` - Code ownership

### 🚀 Next Steps

See [ROADMAP.md](ROADMAP.md) for future plans:

### v0.1.0 - Initial Release (Planned)
- [ ] Final testing and validation
- [ ] Performance benchmarks
- [ ] Security audit
- [ ] GitHub repository setup
- [ ] pkg.go.dev publication
- [ ] Official announcement

### v0.2.0 - Enhanced Features
- [ ] HTTP webhook delivery provider
- [ ] gRPC delivery provider
- [ ] Message encryption
- [ ] Rate limiting
- [ ] Prometheus metrics

### v1.0.0 - Production LTS
- [ ] API stability guarantee
- [ ] Long-term support
- [ ] Production documentation
- [ ] Enterprise features

---

## Links

- **Repository**: https://github.com/coregx/pubsub
- **Documentation**: https://github.com/coregx/pubsub/tree/main/docs
- **API Reference**: https://pkg.go.dev/github.com/coregx/pubsub
- **Issues**: https://github.com/coregx/pubsub/issues
- **Roadmap**: https://github.com/coregx/pubsub/blob/main/ROADMAP.md

---

*Last Updated: 2025-11-24*
