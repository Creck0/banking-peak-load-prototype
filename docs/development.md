# Development Guide

## Prerequisites

- Go 1.25
- Docker & Docker Compose v2
- k6 (for load testing)
- Make (optional, for convenience commands)

## Project Structure

```
banking-peak-load-prototype/
├── CLAUDE.md                  # Claude Code context
├── docker-compose.yml         # All services with profiles
├── .env.baseline              # Feature flags all OFF
├── .env.optimized             # Feature flags all ON
├── .env                       # Active config (gitignored, copy from preset)
├── docs/
│   ├── PRD.md
│   ├── ARCHITECTURE.md
│   ├── DEVELOPMENT.md
│   ├── WORKFLOW.md
│   └── adrs/
│       ├── 001-go-over-rust.md
│       ├── 002-feature-flag-over-branches.md
│       ├── 003-pgbouncer-connection-pooling.md
│       ├── 004-redis-caching-strategy.md
│       └── 005-async-write-via-queue.md
├── cmd/
│   └── server/
│       └── main.go            # Entry point
├── internal/
│   ├── config/                # Env-based configuration
│   ├── handler/               # HTTP handlers per endpoint
│   ├── middleware/             # Rate limiter, circuit breaker, logging, tracing
│   ├── repository/            # DB access (with cache-aside logic)
│   ├── service/               # Business logic
│   ├── queue/                 # Queue producer + consumer/worker
│   └── model/                 # Domain types
├── migrations/                # SQL migrations
├── seeds/                     # Dummy data generation scripts
├── scripts/
│   ├── load-test/             # k6 scripts
│   └── setup/                 # Helper scripts (seed, wait-for-db, etc.)
├── deployments/
│   ├── docker/                # Dockerfiles
│   ├── pgbouncer/             # PgBouncer config
│   ├── prometheus/            # prometheus.yml
│   └── grafana/               # Dashboard JSON provisioning
└── Makefile
```

## Quick Start

```bash
# 1. Clone and setup
cp .env.baseline .env

# 2. Run baseline
docker compose up -d

# 3. Run optimized
cp .env.optimized .env
docker compose --profile optimized up -d

# 4. Run full stack (with observability)
docker compose --profile optimized --profile observability up -d

# 5. Run load test
k6 run scripts/load-test/baseline.js
```

## Environment Variables

### Application
| Var | Default | Description |
|-----|---------|-------------|
| `APP_PORT` | `8080` | HTTP server port |
| `APP_ENV` | `development` | Environment name |

### Feature Flags
| Var | Default | Description |
|-----|---------|-------------|
| `CACHE_ENABLED` | `false` | Enable Redis cache for read path |
| `QUEUE_ENABLED` | `false` | Enable async write via message queue |
| `RATE_LIMIT_ENABLED` | `false` | Enable rate limiting middleware |
| `RATE_LIMIT_RPS` | `100` | Requests per second per client |
| `RATE_LIMIT_BURST` | `200` | Burst allowance |
| `CIRCUIT_BREAKER_ENABLED` | `false` | Enable circuit breaker |
| `CB_MAX_FAILURES` | `5` | Failures before circuit opens |
| `CB_TIMEOUT_SECONDS` | `10` | Duration circuit stays open |
| `DB_READ_REPLICA_ENABLED` | `false` | Route reads to replica |

### Database
| Var | Default | Description |
|-----|---------|-------------|
| `DB_PRIMARY_DSN` | `postgres://...` | Primary PostgreSQL DSN |
| `PGBOUNCER_DSN` | `postgres://...` | PgBouncer write-pool DSN (routes to primary) |
| `PGBOUNCER_READ_DSN` | `postgres://...` | PgBouncer read-pool DSN (routes to replica; required when `DB_READ_REPLICA_ENABLED=true`) |

### Redis
| Var | Default | Description |
|-----|---------|-------------|
| `REDIS_ADDR` | `redis:6379` | Redis address |
| `CACHE_BALANCE_TTL` | `10s` | TTL for balance cache |
| `CACHE_TX_STATUS_TTL` | `30s` | TTL for completed tx status |

### Queue
| Var | Default | Description |
|-----|---------|-------------|
| `QUEUE_URL` | `amqp://...` | RabbitMQ connection URL |
| `QUEUE_WORKERS` | `10` | Number of concurrent consumers |

## Coding Conventions

- **Router:** echo
- **DB driver:** pgx/v5 (connect through PgBouncer)
- **Config:** env vars loaded via `caarlos0/env` or `kelseyhightower/envconfig`
- **Errors:** Wrap with context, don't swallow
- **Logging:** `slog` (stdlib) with JSON output, always include `trace_id`
- **Metrics:** `prometheus/client_golang`, register in `init()` or dependency injection
- **Naming:** snake_case for JSON fields, camelCase for Go

## Testing

```bash
# Unit tests
go test ./...

# Integration tests (requires docker compose up)
go test -tags=integration ./...

# Load tests
k6 run scripts/load-test/baseline.js
k6 run scripts/load-test/optimized.js
```

## Dummy Data

Seed script generates:
- 100K accounts with random balances (1M-100M IDR range)
- 1-5M historical transactions across those accounts
- Realistic distribution of transaction statuses (completed, pending, failed)

```bash
go run ./seeds/main.go --accounts=100000 --transactions=1000000
```
