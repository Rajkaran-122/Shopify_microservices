# Enterprise E-Commerce Platform

**PROPRIETARY & CONFIDENTIAL — Digital Metro**

A FAANG-grade, production-ready microservices e-commerce platform engineered for global scale, five-nines availability, and enterprise compliance.

---

## Architecture at a Glance

```
┌─────────────────────────────────────────────────────────────────┐
│                    EDGE LAYER                                    │
│  CloudFront CDN (400+ PoPs) → WAF → AWS Shield                 │
├─────────────────────────────────────────────────────────────────┤
│                    API GATEWAY (:8080)                            │
│  Rate Limiting │ JWT Auth │ Circuit Breaker │ Request Routing    │
├──────────┬──────────┬──────────┬──────────┬─────────────────────┤
│ Identity │ Catalog  │ Commerce │ Payment  │ Platform            │
│ Domain   │ Domain   │ Domain   │ CDE      │ Services            │
│          │          │          │ (PCI)    │                     │
│ User     │ Product  │ Cart     │ Payment  │ Notification        │
│ Service  │ Service  │ Service  │ Service  │ Service             │
│ :50051   │ :50052   │ :50057   │ :50055   │ :50054              │
│          │ Inventory│ Order    │          │                     │
│          │ Service  │ Service  │          │                     │
│          │ :50056   │ :50053   │          │                     │
├──────────┴──────────┴──────────┴──────────┴─────────────────────┤
│                    EVENT BUS (Apache Kafka)                       │
│  Saga Choreography │ CQRS Projections │ Event Sourcing          │
├─────────────────────────────────────────────────────────────────┤
│                    DATA LAYER                                    │
│  PostgreSQL (Aurora) │ Redis Cluster │ Elasticsearch             │
└─────────────────────────────────────────────────────────────────┘
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Language** | Go 1.21+ |
| **Communication** | gRPC + Protocol Buffers |
| **API Gateway** | Custom (Chi router) with Kong upgrade path |
| **Event Streaming** | Apache Kafka (AWS MSK) |
| **Databases** | PostgreSQL 16 (Aurora), Redis 7, Elasticsearch |
| **Orchestration** | Kubernetes (EKS), Helm, ArgoCD |
| **Observability** | OpenTelemetry, Prometheus, Grafana, Jaeger |
| **CI/CD** | GitHub Actions → ArgoCD (GitOps) |
| **IaC** | Terraform |
| **Security** | Keycloak (OIDC), HashiCorp Vault, Istio mTLS |

## Project Structure

```
├── .github/workflows/       # CI/CD pipelines (lint, test, scan, deploy)
├── docs/
│   ├── adr/                 # Architecture Decision Records
│   ├── architecture/        # C4 diagrams and system design
│   └── BRD.md              # Business Requirements Document
├── kubernetes/
│   └── helm/               # Helm charts for K8s deployment
├── observability/
│   └── prometheus/          # Prometheus configs & SLO alert rules
├── pkg/                     # Shared platform libraries
│   ├── circuitbreaker/      # Circuit breaker pattern
│   ├── config/              # 12-Factor config loader
│   ├── events/              # Kafka event framework (CloudEvents)
│   ├── health/              # K8s liveness/readiness probes
│   ├── logger/              # Structured JSON logging + tracing
│   ├── middleware/          # JWT auth, rate limiting, CORS
│   ├── retry/              # Exponential backoff with jitter
│   └── tracer/             # OpenTelemetry initialization
├── proto/                   # Protocol Buffer definitions
│   ├── cart/               # Shopping cart domain
│   ├── inventory/          # Inventory & reservations
│   ├── notification/       # Notifications
│   ├── order/              # Order management
│   ├── payment/            # Payment processing (PCI)
│   ├── product/            # Product catalog
│   └── user/               # Identity & access
├── services/                # Microservice implementations
│   ├── api-gateway/        # HTTP → gRPC translation layer
│   ├── cart-service/       # Redis-backed cart (90-day TTL)
│   ├── inventory-service/  # Reservation engine
│   ├── notification-service/# Event-driven notifications
│   ├── order-service/      # CQRS + Saga orchestration
│   ├── payment-service/    # Stripe integration + fraud scoring
│   ├── product-service/    # Catalog with Redis caching
│   └── user-service/       # OAuth2/OIDC + JWT + RBAC
├── terraform/               # AWS infrastructure as code
├── tests/
│   └── load/k6/           # k6 performance regression tests
├── docker-compose.yml       # Full local stack (services + infra + observability)
└── LICENSE                  # Proprietary — All Rights Reserved
```

## System Design Patterns

| Pattern | Implementation | BRD Reference |
|---------|---------------|---------------|
| **Circuit Breaker** | `pkg/circuitbreaker/` — 50% failure threshold, 30s open state | Section 7.1 |
| **Saga (Choreography)** | Kafka event-driven order placement flow | Section 7.1 |
| **Transactional Outbox** | DB + outbox table + CDC → Kafka | Section 7.1 |
| **CQRS** | Separate read/write models for order domain | Section 7.2 |
| **Rate Limiting** | Token bucket — 100/min unauth, 5K/min auth | Section 7.4 |
| **Retry + Backoff** | Exponential (100ms base, 2x multiplier, 30s max) | Section 7.5 |
| **Cache-Aside** | Redis L2 cache with TTL per entity type | Section 7.3 |

## Quick Start

```bash
# Clone and start the full stack
docker-compose up -d

# Verify health
curl http://localhost:8080/health

# Access observability
# Grafana:    http://localhost:3000 (admin/admin)
# Jaeger:     http://localhost:16686
# Prometheus: http://localhost:9090
```

## SLO Targets

| Service | SLO | SLA |
|---------|-----|-----|
| API Gateway | 99.999% | 99.99% |
| Payment Service | 99.999% | 99.99% |
| User Auth | 99.999% | 99.99% |
| Order Management | 99.99% | 99.95% |
| Product Catalog | 99.99% | 99.95% |
| Notification | 99.9% | 99.5% |

## Performance Targets

| Metric | Target |
|--------|--------|
| API P50 | < 20ms |
| API P99 | < 100ms |
| Checkout P99 | < 200ms |
| Search P99 | < 50ms |
| Peak Concurrent Users | 10,000,000 |
| Transactions/Second | 500,000+ |

---

## Copyright & License

**Copyright © 2026 Digital Metro. All Rights Reserved.**

This repository is strictly proprietary. Unauthorized access, copying, or distribution is prohibited.
