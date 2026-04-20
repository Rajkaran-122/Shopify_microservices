// k6 Load Test — Checkout Flow Performance Regression
// BRD Section 12.1: k6 soak/spike/stress tests per staging deploy
// Targets: P99 checkout < 200ms, P99 API < 100ms, P50 < 20ms (BRD Section 2.2)

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ---- Custom Metrics (BRD KPIs) ----
const checkoutDuration = new Trend('checkout_duration', true);
const searchDuration = new Trend('search_duration', true);
const errorRate = new Rate('errors');

// ---- Test Configuration ----
export const options = {
  scenarios: {
    // Soak test: 2h at 70% load (BRD Section 12.1)
    soak: {
      executor: 'constant-vus',
      vus: 100,
      duration: '2h',
      exec: 'soakTest',
    },
    // Spike test: 2x baseline (BRD Section 12.1)
    spike: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 50 },   // Ramp to normal
        { duration: '5m', target: 50 },   // Hold at normal
        { duration: '30s', target: 200 },  // Spike to 4x
        { duration: '3m', target: 200 },   // Hold spike
        { duration: '2m', target: 50 },   // Scale down
        { duration: '3m', target: 50 },   // Recovery
        { duration: '1m', target: 0 },    // Wind down
      ],
      exec: 'spikeTest',
      startTime: '2h10m',
    },
  },

  // BRD Thresholds (Section 2.2 KPIs)
  thresholds: {
    'http_req_duration': ['p(50)<20', 'p(95)<50', 'p(99)<100'],     // API P50/P95/P99
    'checkout_duration': ['p(99)<200'],                              // Checkout P99 < 200ms
    'search_duration': ['p(99)<50'],                                 // Search P99 < 50ms
    'errors': ['rate<0.001'],                                        // Error rate < 0.1%
    'http_req_failed': ['rate<0.01'],                                // HTTP failure < 1%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// ---- Soak Test: Normal Load (BRD: 100K QPS) ----
export function soakTest() {
  group('User Registration & Login', () => {
    const uniqueId = `user_${Date.now()}_${__VU}`;

    // Register
    const registerRes = http.post(`${BASE_URL}/api/users/register`, JSON.stringify({
      email: `${uniqueId}@loadtest.com`,
      username: uniqueId,
      password: 'LoadTest_Pass123!',
      first_name: 'Load',
      last_name: 'Tester',
    }), { headers: { 'Content-Type': 'application/json' } });

    check(registerRes, {
      'register status 200': (r) => r.status === 200,
      'register has user': (r) => JSON.parse(r.body).success === true,
    }) || errorRate.add(1);

    // Login
    const loginRes = http.post(`${BASE_URL}/api/users/login`, JSON.stringify({
      email: `${uniqueId}@loadtest.com`,
      password: 'LoadTest_Pass123!',
    }), { headers: { 'Content-Type': 'application/json' } });

    check(loginRes, {
      'login status 200': (r) => r.status === 200,
      'login has token': (r) => JSON.parse(r.body).token !== undefined,
    }) || errorRate.add(1);
  });

  group('Product Search & Discovery', () => {
    const searchStart = Date.now();
    const searchRes = http.get(`${BASE_URL}/api/products?page=1&limit=20`);
    searchDuration.add(Date.now() - searchStart);

    check(searchRes, {
      'search status 200': (r) => r.status === 200,
      'search has products': (r) => JSON.parse(r.body).products !== undefined,
    }) || errorRate.add(1);
  });

  group('Checkout Flow', () => {
    const checkoutStart = Date.now();

    // Create order (simulates full checkout)
    const orderRes = http.post(`${BASE_URL}/api/orders`, JSON.stringify({
      user_id: 'load-test-user',
      items: [
        { product_id: 'p1', quantity: 2, price: 99.99 },
        { product_id: 'p2', quantity: 1, price: 199.99 },
      ],
    }), { headers: { 'Content-Type': 'application/json' } });

    checkoutDuration.add(Date.now() - checkoutStart);

    check(orderRes, {
      'checkout status 200': (r) => r.status === 200,
      'order created': (r) => JSON.parse(r.body).success === true,
    }) || errorRate.add(1);
  });

  group('Health Check', () => {
    const healthRes = http.get(`${BASE_URL}/health`);
    check(healthRes, {
      'health check OK': (r) => r.status === 200,
    });
  });

  sleep(1); // Think time between iterations
}

// ---- Spike Test: Sudden Traffic Surge ----
export function spikeTest() {
  // Simplified high-frequency requests to test auto-scaling
  const res = http.get(`${BASE_URL}/api/products?page=1&limit=10`);

  check(res, {
    'spike: status 200': (r) => r.status === 200,
    'spike: not rate limited': (r) => r.status !== 429,
  }) || errorRate.add(1);

  sleep(0.1); // Minimal think time for spike pressure
}

// ---- Test Summary ----
export function handleSummary(data) {
  return {
    'stdout': textSummary(data),
    'load-test-results.json': JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  return `
=== FAANG Load Test Results ===
BRD KPI Validation:

API P50 Latency:      ${data.metrics.http_req_duration?.values?.['p(50)']?.toFixed(2) || 'N/A'}ms (target: < 20ms)
API P99 Latency:      ${data.metrics.http_req_duration?.values?.['p(99)']?.toFixed(2) || 'N/A'}ms (target: < 100ms)
Checkout P99:         ${data.metrics.checkout_duration?.values?.['p(99)']?.toFixed(2) || 'N/A'}ms (target: < 200ms)
Search P99:           ${data.metrics.search_duration?.values?.['p(99)']?.toFixed(2) || 'N/A'}ms (target: < 50ms)
Error Rate:           ${(data.metrics.errors?.values?.rate * 100)?.toFixed(4) || 'N/A'}% (target: < 0.1%)
Total Requests:       ${data.metrics.http_reqs?.values?.count || 'N/A'}
================================
`;
}
