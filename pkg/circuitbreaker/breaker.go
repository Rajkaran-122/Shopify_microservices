// Package circuitbreaker implements the Circuit Breaker resilience pattern.
// Prevents cascading failures by wrapping remote calls with fail-fast logic.
// Configuration per BRD Section 7.1: failure rate threshold 50%,
// slow call rate threshold 80%, wait duration in open state 30s.
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation — requests flow through
	StateOpen                  // Tripped — requests fail immediately
	StateHalfOpen              // Testing — limited requests allowed to probe recovery
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

var (
	ErrCircuitOpen    = errors.New("circuit breaker is open")
	ErrTooManyFails   = errors.New("too many failures, circuit opened")
)

// Config defines the circuit breaker behavior per BRD specifications.
type Config struct {
	Name                   string
	FailureRateThreshold   float64       // 0.5 = 50% (BRD: 50%)
	SlowCallRateThreshold  float64       // 0.8 = 80% (BRD: 80%)
	SlowCallDuration       time.Duration // Calls slower than this are "slow"
	WaitDurationInOpenState time.Duration // BRD: 30s
	PermittedCallsInHalfOpen int          // BRD: 5
	MinimumNumberOfCalls   int           // Minimum calls before evaluating
	SlidingWindowSize      int           // Number of calls in evaluation window
}

// DefaultConfig returns a circuit breaker config matching BRD Section 7.1.
func DefaultConfig(name string) Config {
	return Config{
		Name:                    name,
		FailureRateThreshold:    0.50,
		SlowCallRateThreshold:   0.80,
		SlowCallDuration:        2 * time.Second,
		WaitDurationInOpenState: 30 * time.Second,
		PermittedCallsInHalfOpen: 5,
		MinimumNumberOfCalls:    10,
		SlidingWindowSize:       100,
	}
}

// Breaker implements the circuit breaker pattern with sliding window metrics.
type Breaker struct {
	mu              sync.Mutex
	config          Config
	state           State
	failures        int
	successes       int
	slowCalls       int
	totalCalls      int
	halfOpenCalls   int
	lastStateChange time.Time
	onStateChange   func(name string, from, to State)
}

// New creates a new circuit breaker with the given configuration.
func New(cfg Config) *Breaker {
	return &Breaker{
		config:          cfg,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// OnStateChange registers a callback for state transitions (for alerting/metrics).
func (b *Breaker) OnStateChange(fn func(name string, from, to State)) {
	b.onStateChange = fn
}

// Execute wraps a function call with circuit breaker protection.
// If the circuit is open, returns ErrCircuitOpen immediately (fail-fast).
// Tracks success/failure metrics and transitions states accordingly.
func (b *Breaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	b.mu.Lock()

	switch b.state {
	case StateOpen:
		// Check if wait duration has elapsed — transition to half-open
		if time.Since(b.lastStateChange) >= b.config.WaitDurationInOpenState {
			b.transitionTo(StateHalfOpen)
		} else {
			b.mu.Unlock()
			return ErrCircuitOpen
		}

	case StateHalfOpen:
		// Only allow permitted number of probe calls
		if b.halfOpenCalls >= b.config.PermittedCallsInHalfOpen {
			b.mu.Unlock()
			return ErrCircuitOpen
		}
		b.halfOpenCalls++
	}

	b.mu.Unlock()

	// Execute the actual call with timing
	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalCalls++

	if err != nil {
		b.failures++
		b.evaluateState()
		return err
	}

	// Track slow calls
	if duration > b.config.SlowCallDuration {
		b.slowCalls++
	}

	b.successes++

	// If in half-open and call succeeded, try to close the circuit
	if b.state == StateHalfOpen && b.successes >= b.config.PermittedCallsInHalfOpen {
		b.transitionTo(StateClosed)
	}

	b.evaluateState()
	return nil
}

// evaluateState checks metrics and transitions state if thresholds breached.
func (b *Breaker) evaluateState() {
	if b.totalCalls < b.config.MinimumNumberOfCalls {
		return
	}

	failureRate := float64(b.failures) / float64(b.totalCalls)
	slowCallRate := float64(b.slowCalls) / float64(b.totalCalls)

	switch b.state {
	case StateClosed:
		if failureRate >= b.config.FailureRateThreshold || slowCallRate >= b.config.SlowCallRateThreshold {
			b.transitionTo(StateOpen)
		}

	case StateHalfOpen:
		if failureRate >= b.config.FailureRateThreshold {
			b.transitionTo(StateOpen)
		}
	}

	// Reset sliding window if it exceeds the configured size
	if b.totalCalls >= b.config.SlidingWindowSize {
		b.resetCounters()
	}
}

// transitionTo changes the circuit breaker state and fires callbacks.
func (b *Breaker) transitionTo(newState State) {
	oldState := b.state
	b.state = newState
	b.lastStateChange = time.Now()
	b.resetCounters()

	if b.onStateChange != nil {
		go b.onStateChange(b.config.Name, oldState, newState)
	}
}

func (b *Breaker) resetCounters() {
	b.failures = 0
	b.successes = 0
	b.slowCalls = 0
	b.totalCalls = 0
	b.halfOpenCalls = 0
}

// State returns the current circuit breaker state.
func (b *Breaker) GetState() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// String returns a human-readable representation of the circuit breaker.
func (b *Breaker) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return fmt.Sprintf("CircuitBreaker[%s] state=%s failures=%d/%d",
		b.config.Name, b.state, b.failures, b.totalCalls)
}
