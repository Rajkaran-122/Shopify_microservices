// Package health provides standardized liveness, readiness, and startup probe
// handlers for Kubernetes health checks across all microservices.
// Implements BRD Section 7.5 Health Checks requirement.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a service or dependency.
type Status string

const (
	StatusUp      Status = "UP"
	StatusDown    Status = "DOWN"
	StatusDegraded Status = "DEGRADED"
)

// Check is a function that verifies a dependency's health.
type Check func(ctx context.Context) error

// Checker manages health check registrations and overall status.
type Checker struct {
	mu          sync.RWMutex
	checks      map[string]Check
	ready       bool
	startupDone bool
}

// Response is the JSON health check response body.
type Response struct {
	Status      Status            `json:"status"`
	Service     string            `json:"service"`
	Version     string            `json:"version"`
	Uptime      string            `json:"uptime"`
	Timestamp   string            `json:"timestamp"`
	Checks      map[string]string `json:"checks,omitempty"`
}

var startTime = time.Now()

// NewChecker creates a new health checker.
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]Check),
	}
}

// AddCheck registers a named health check (e.g., "postgres", "redis", "kafka").
func (c *Checker) AddCheck(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// SetReady marks the service as ready to receive traffic.
// Should be called after all dependencies are initialized.
func (c *Checker) SetReady(ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ready = ready
}

// SetStartupDone marks the startup phase as complete.
func (c *Checker) SetStartupDone() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startupDone = true
}

// LivenessHandler returns HTTP handler for Kubernetes liveness probe.
// Returns 200 if the process is alive; 503 otherwise.
// Purpose: "Is the process alive?" — restart if not.
func (c *Checker) LivenessHandler(serviceName, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := Response{
			Status:    StatusUp,
			Service:   serviceName,
			Version:   version,
			Uptime:    time.Since(startTime).String(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

// ReadinessHandler returns HTTP handler for Kubernetes readiness probe.
// Returns 200 only if all registered checks pass.
// Purpose: "Is the service ready to handle traffic?" — remove from LB if not.
func (c *Checker) ReadinessHandler(serviceName, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.mu.RLock()
		isReady := c.ready
		checks := make(map[string]Check, len(c.checks))
		for k, v := range c.checks {
			checks[k] = v
		}
		c.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		resp := Response{
			Service:   serviceName,
			Version:   version,
			Uptime:    time.Since(startTime).String(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Checks:    make(map[string]string),
		}

		if !isReady {
			resp.Status = StatusDown
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(resp)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		allHealthy := true
		for name, check := range checks {
			if err := check(ctx); err != nil {
				resp.Checks[name] = "DOWN: " + err.Error()
				allHealthy = false
			} else {
				resp.Checks[name] = "UP"
			}
		}

		if allHealthy {
			resp.Status = StatusUp
			w.WriteHeader(http.StatusOK)
		} else {
			resp.Status = StatusDegraded
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	}
}

// StartupHandler returns HTTP handler for Kubernetes startup probe.
// Returns 200 once startup is complete.
// Purpose: Prevents liveness/readiness checks during slow startup.
func (c *Checker) StartupHandler(serviceName, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.mu.RLock()
		done := c.startupDone
		c.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		resp := Response{
			Service:   serviceName,
			Version:   version,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}

		if done {
			resp.Status = StatusUp
			w.WriteHeader(http.StatusOK)
		} else {
			resp.Status = StatusDown
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	}
}
