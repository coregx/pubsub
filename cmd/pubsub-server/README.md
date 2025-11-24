# PubSub Server

Production-ready standalone PubSub server with REST API.

## Features

✅ **REST API** - Publish, subscribe, manage subscriptions
✅ **Multi-Database** - MySQL, PostgreSQL, SQLite support
✅ **Docker Ready** - Dockerfile + docker-compose included
✅ **12-Factor App** - Configuration via environment variables
✅ **Graceful Shutdown** - Proper signal handling
✅ **Health Checks** - `/api/v1/health` endpoint
✅ **Production Ready** - Based on Relica (zero dependencies)

## Quick Start with Docker

```bash
# Start MySQL + PubSub Server
docker-compose up -d

# Check health
curl http://localhost:8080/api/v1/health

# View logs
docker-compose logs -f pubsub-server
```

## Quick Start (Local)

```bash
# Set environment variables
export DB_DRIVER=mysql
export DB_HOST=localhost
export DB_PORT=3306
export DB_USER=pubsub
export DB_PASSWORD=your_password
export DB_NAME=pubsub

# Run server
go run main.go
```

## Configuration

All configuration via environment variables (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | HTTP server host |
| `SERVER_PORT` | `8080` | HTTP server port |
| `DB_DRIVER` | `mysql` | Database driver (mysql/postgres/sqlite3) |
| `DB_HOST` | `localhost` | Database host |
| `DB_PORT` | `3306` | Database port |
| `DB_USER` | `pubsub` | Database user |
| `DB_PASSWORD` | **required** | Database password |
| `DB_NAME` | `pubsub` | Database name |
| `DB_PREFIX` | `pubsub_` | Table prefix |
| `PUBSUB_BATCH_SIZE` | `100` | Worker batch size |
| `PUBSUB_WORKER_INTERVAL` | `30` | Worker interval (seconds) |
| `PUBSUB_ENABLE_NOTIFICATIONS` | `true` | Enable notifications |

## API Endpoints

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
Content-Type: application/json

{
  "subscriberId": 1,
  "topicCode": "user.signup",
  "identifier": "optional-identifier"
}
```

### List Subscriptions
```bash
GET /api/v1/subscriptions?subscriberId=1&identifier=optional
```

### Unsubscribe
```bash
DELETE /api/v1/subscriptions/123
```

### Health Check
```bash
GET /api/v1/health
```

## Architecture

```
┌─────────────┐
│  REST API   │  ← HTTP Handlers
└──────┬──────┘
       │
┌──────▼────────────────────┐
│  Services                 │
│  - Publisher              │  ← Business Logic
│  - SubscriptionManager    │
│  - QueueWorker            │
└──────┬────────────────────┘
       │
┌──────▼────────────────────┐
│  Relica Adapters          │  ← Data Access
│  (Zero dependencies!)     │
└──────┬────────────────────┘
       │
┌──────▼────────────────────┐
│  MySQL / PostgreSQL       │  ← Database
└───────────────────────────┘
```

## Development

```bash
# Install dependencies
go mod download

# Build
go build .

# Run
./pubsub-server

# Build Docker image
docker build -t pubsub-server -f Dockerfile ../..

# Run with docker-compose
docker-compose up
```

## Production Deployment

1. **Set environment variables** (use secrets management)
2. **Run migrations** (from `/migrations` directory)
3. **Deploy container** with proper health checks
4. **Configure reverse proxy** (nginx/traefik) for HTTPS
5. **Monitor** `/api/v1/health` endpoint

## License

Same as parent project (MIT or your choice)
