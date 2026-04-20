// Package events provides Kafka producer and consumer wrappers with
// OpenTelemetry trace propagation for event-driven architecture.
// Implements BRD Section 4.1 (Event-Driven Asynchrony) and
// ADR-001 (choreography-based saga with transactional outbox).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ---- Event Types (BRD Section 4.3 Domain Map) ----

// Topic naming convention: {domain}.{entity}.{event}
const (
	// Order domain events (Saga choreography)
	TopicOrderCreated    = "order.order.created"
	TopicOrderConfirmed  = "order.order.confirmed"
	TopicOrderCancelled  = "order.order.cancelled"
	TopicOrderFailed     = "order.order.failed"

	// Payment domain events
	TopicPaymentSucceeded = "payment.transaction.succeeded"
	TopicPaymentFailed    = "payment.transaction.failed"
	TopicPaymentRefunded  = "payment.transaction.refunded"

	// Inventory domain events
	TopicInventoryReserved  = "inventory.reservation.created"
	TopicInventoryConfirmed = "inventory.reservation.confirmed"
	TopicInventoryReleased  = "inventory.reservation.released"
	TopicStockUpdated       = "inventory.stock.updated"

	// Notification domain events
	TopicNotificationSend = "notification.message.send"

	// User domain events
	TopicUserRegistered = "user.account.registered"
	TopicUserUpdated    = "user.account.updated"
)

// Event represents a domain event published to Kafka.
// All events follow CloudEvents specification for interoperability.
type Event struct {
	// CloudEvents metadata
	ID              string                 `json:"id"`
	Source          string                 `json:"source"`           // Service that produced the event
	Type            string                 `json:"type"`             // Event type (topic name)
	Time            string                 `json:"time"`             // ISO 8601 timestamp
	DataContentType string                 `json:"datacontenttype"`  // application/json

	// Business data
	Data            interface{}            `json:"data"`

	// Tracing context (BRD NFR-OBS-001)
	TraceID         string                 `json:"trace_id,omitempty"`
	SpanID          string                 `json:"span_id,omitempty"`

	// Saga correlation
	CorrelationID   string                 `json:"correlation_id"`   // Links saga steps
	CausationID     string                 `json:"causation_id"`     // ID of event that caused this

	// Idempotency
	IdempotencyKey  string                 `json:"idempotency_key"`

	// Metadata
	Metadata        map[string]string      `json:"metadata,omitempty"`
}

// NewEvent creates a new domain event with proper metadata.
func NewEvent(source, eventType string, data interface{}) *Event {
	return &Event{
		ID:              generateEventID(),
		Source:          source,
		Type:            eventType,
		Time:            time.Now().UTC().Format(time.RFC3339Nano),
		DataContentType: "application/json",
		Data:            data,
		IdempotencyKey:  generateEventID(),
		Metadata:        make(map[string]string),
	}
}

// WithCorrelation sets saga correlation tracking IDs.
func (e *Event) WithCorrelation(correlationID, causationID string) *Event {
	e.CorrelationID = correlationID
	e.CausationID = causationID
	return e
}

// Marshal serializes the event to JSON bytes for Kafka.
func (e *Event) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEvent deserializes a JSON event from Kafka.
func UnmarshalEvent(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}
	return &event, nil
}

// ---- Saga Event Data Structs ----

// OrderCreatedData is the payload for TopicOrderCreated events.
type OrderCreatedData struct {
	OrderID     string       `json:"order_id"`
	UserID      string       `json:"user_id"`
	Items       []OrderItem  `json:"items"`
	TotalAmount float64      `json:"total_amount"`
	Currency    string       `json:"currency"`
}

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int32   `json:"quantity"`
	Price     float64 `json:"price"`
}

// InventoryReservedData is the payload for TopicInventoryReserved events.
type InventoryReservedData struct {
	OrderID       string `json:"order_id"`
	ReservationID string `json:"reservation_id"`
	WarehouseID   string `json:"warehouse_id"`
	ExpiresAt     string `json:"expires_at"`
}

// PaymentSucceededData is the payload for TopicPaymentSucceeded events.
type PaymentSucceededData struct {
	OrderID       string  `json:"order_id"`
	PaymentID     string  `json:"payment_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	GatewayTxnID  string  `json:"gateway_txn_id"`
}

// PaymentFailedData is the payload for TopicPaymentFailed events.
type PaymentFailedData struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
	Code    string `json:"error_code"`
}

// ---- Outbox Pattern (BRD Section 7.1) ----

// OutboxEntry represents a row in the transactional outbox table.
// Written in the same DB transaction as the business data change.
// Debezium CDC connector reads this table and publishes to Kafka.
type OutboxEntry struct {
	ID              string    `json:"id" db:"id"`
	AggregateType   string    `json:"aggregate_type" db:"aggregate_type"`
	AggregateID     string    `json:"aggregate_id" db:"aggregate_id"`
	EventType       string    `json:"event_type" db:"event_type"`
	Payload         []byte    `json:"payload" db:"payload"`
	CorrelationID   string    `json:"correlation_id" db:"correlation_id"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	Published       bool      `json:"published" db:"published"`
}

// ---- Producer/Consumer Interfaces ----

// Producer publishes events to Kafka topics.
type Producer interface {
	Publish(ctx context.Context, topic string, key string, event *Event) error
	Close() error
}

// Consumer subscribes to Kafka topics and processes events.
type Consumer interface {
	Subscribe(ctx context.Context, topics []string, handler EventHandler) error
	Close() error
}

// EventHandler processes a single consumed event.
type EventHandler func(ctx context.Context, event *Event) error

// ---- Helpers ----

func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
