# Architecture Decision Record: Event-Driven Architecture
# ADR-001 | Status: ACCEPTED | Date: 2026-04-20

## Context

The current e-commerce platform uses synchronous gRPC communication between all microservices. When the Order Service creates an order, it synchronously calls the User Service (validate user), Product Service (check inventory), and Notification Service (send confirmation). This creates tight temporal coupling, cascading failure risk, and inability to scale services independently.

BRD Section 3.1 identifies this as a critical gap: "No event streaming infrastructure. Services communicate synchronously, creating cascading failure risk and tight temporal coupling."

## Decision

**We will adopt Apache Kafka as our event streaming backbone and migrate all cross-domain workflows to choreography-based event-driven architecture.**

### Key Design Choices:

1. **Choreography over Orchestration**: Each service reacts to events independently rather than a central orchestrator managing the workflow. This maximizes service autonomy and reduces single points of failure.

2. **Transactional Outbox Pattern**: Services write domain events to an `outbox` table within the same database transaction as the business data change. A Debezium CDC connector reads the outbox and publishes to Kafka, guaranteeing exactly-once event delivery without distributed transactions.

3. **Saga Pattern for Order Placement**: The critical Order Placement flow uses a choreography-based saga:
   - `OrderCreated` → Inventory Service reserves stock
   - `InventoryReserved` → Payment Service charges customer
   - `PaymentSucceeded` → Order Service confirms order
   - `OrderConfirmed` → Notification Service sends confirmation
   - Compensating: `PaymentFailed` → Inventory releases reservation

4. **Topic Naming Convention**: `{domain}.{entity}.{event}` (e.g., `order.order.created`, `payment.transaction.succeeded`)

5. **Schema Registry**: Confluent Schema Registry with Avro for event schema evolution with backward/forward compatibility guarantees.

## Consequences

### Positive
- Services are temporally decoupled — Order Service does not wait for Notification Service
- Individual service failures do not cascade to upstream services
- Events provide a natural audit trail and can be replayed for disaster recovery
- Enables CQRS: events drive read-model projections into Elasticsearch and Redis

### Negative
- Eventual consistency introduces complexity in UI design (must handle "processing" states)
- Debugging distributed workflows requires distributed tracing (mitigated by OpenTelemetry)
- Kafka adds operational complexity (managed via AWS MSK in production)

### Risks
- Event ordering: Mitigated by partitioning on business key (user_id, order_id)
- Message loss: Mitigated by replication factor 3, min.insync.replicas 2
- Duplicate processing: Mitigated by idempotency keys on all event handlers

## Alternatives Considered

1. **gRPC-only synchronous**: Rejected — BRD requirement for 99.999% availability impossible with synchronous call chains
2. **AWS SQS/SNS**: Rejected — lacks ordering guarantees, replay capability, and schema registry
3. **RabbitMQ**: Rejected — Kafka's log-based architecture provides event replay for CQRS, better scalability for 5M events/second target
