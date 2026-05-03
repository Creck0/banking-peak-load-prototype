# Implementation Plan

## Phase 1 — Baseline
- [x] Seed data generator (`seeds/main.go`) — 100K accounts, 1M transactions
- [x] Real balance validation + atomic debit/credit on transaction creation
- [ ] k6 baseline load test script (`scripts/load-test/baseline.js`)

## Phase 2 — Optimizations
- [x] Redis cache-aside for GET /accounts/:id/balance and GET /transactions/:id/status
- [x] Cache invalidation on writes
- [ ] RabbitMQ producer — publish transaction to queue (return 202 + pending)
- [ ] RabbitMQ consumer / worker — process queued transactions, update DB
- [ ] Circuit breaker middleware (`sony/gobreaker`) wrapping DB/cache/queue calls
- [ ] Route reads to PostgreSQL replica when `DB_READ_REPLICA_ENABLED=true`

## Phase 3 — Observability
- [ ] Prometheus scrape config (`deployments/prometheus/prometheus.yml`)
- [ ] Grafana dashboard provisioning (TPS, p95 latency, error rate, cache hit rate)
- [ ] k6 optimized load test script (`scripts/load-test/optimized.js`)
