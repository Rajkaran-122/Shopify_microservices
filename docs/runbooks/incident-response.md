# Incident Response Runbook — P0/P1 Incidents
# BRD Section 15.2: Blameless post-mortems within 48 hours

## Severity Definitions

| Severity | Description | Response Time | Example |
|----------|-------------|---------------|---------|
| **P0 — Critical** | Platform-wide outage or data loss | Bridge within 5 min | Payment service down, data corruption |
| **P1 — High** | Major feature degraded affecting >10% users | Acknowledge within 15 min | Search unavailable, checkout >5s latency |
| **P2 — Medium** | Minor feature degraded affecting <10% users | Acknowledge within 1 hour | Notifications delayed, dashboard slow |
| **P3 — Low** | Cosmetic or non-customer-facing issue | Next business day | Admin tool UI glitch, log formatting |

---

## P0 Incident Response Procedure

### 1. Detection (Target: < 2 minutes — BRD NFR-OBS-004)
- [ ] SLO burn-rate alert fired in PagerDuty
- [ ] Verify alert is legitimate (not a false positive)
- [ ] Acknowledge the incident in PagerDuty

### 2. Triage (Target: < 5 minutes)
- [ ] Open war room bridge (Zoom/Slack)
- [ ] Assign roles:
  - **Incident Commander (IC):** Coordinates response, makes decisions
  - **Tech Lead:** Investigates root cause, proposes fixes
  - **Communications Lead:** Updates status page, notifies stakeholders
- [ ] Determine blast radius: which services, how many users affected?

### 3. Diagnosis
- [ ] Check Grafana golden-signals dashboard: which signal is anomalous?
  - **Latency spike?** → Check recent deployments, database performance
  - **Error rate spike?** → Check Jaeger traces for failing requests
  - **Traffic anomaly?** → Check for DDoS, check upstream dependencies
  - **Saturation?** → Check CPU/memory/disk, check HPA scaling
- [ ] Check Jaeger distributed traces for error patterns
- [ ] Check Loki logs: filter by `level=error` and `service=<affected>`
- [ ] Check recent deployments: `kubectl rollout history deployment/<service>`
- [ ] Check infrastructure: RDS metrics, Redis cluster health, Kafka lag

### 4. Mitigation (Target: MTTR < 5 minutes — BRD NFR-REL-002)

#### If caused by a bad deployment:
```bash
# Immediate rollback
kubectl rollout undo deployment/<service-name> -n ecommerce

# Verify rollback
kubectl rollout status deployment/<service-name> -n ecommerce
```

#### If caused by database issues:
```bash
# Check connection pool exhaustion
kubectl exec -it <pod> -- psql -c "SELECT count(*) FROM pg_stat_activity;"

# Failover to read replica if primary is degraded
# Update service config to route reads to replica
```

#### If caused by Kafka:
```bash
# Check consumer lag
kafka-consumer-groups --bootstrap-server kafka:9092 --describe --all-groups

# Reset consumer offset if stuck
kafka-consumer-groups --bootstrap-server kafka:9092 --group <group> --reset-offsets --to-latest --execute --topic <topic>
```

#### If caused by DDoS:
- Enable AWS Shield Advanced countermeasures
- Tighten WAF rate limiting rules
- Enable CloudFront under-attack mode

### 5. Resolution
- [ ] Verify all golden signals return to normal
- [ ] Verify SLO burn rate returns to acceptable level
- [ ] Run smoke tests against production
- [ ] Update status page: "Resolved"
- [ ] Notify stakeholders

### 6. Post-Mortem (Required within 48 hours — BRD Section 15.2)

**Template:**

```markdown
## Post-Mortem: [Incident Title]

**Date:** YYYY-MM-DD
**Duration:** X minutes
**Severity:** P0/P1
**IC:** [Name]
**Status:** Resolved

### Impact
- Users affected: X
- Revenue impact: $X
- SLO burn: X% of monthly error budget consumed

### Timeline
- HH:MM — Alert fired
- HH:MM — IC acknowledged
- HH:MM — Root cause identified
- HH:MM — Mitigation applied
- HH:MM — Service fully recovered

### Root Cause (5 Whys)
1. Why did the service fail? → [Answer]
2. Why did [1] happen? → [Answer]
3. Why did [2] happen? → [Answer]
4. Why did [3] happen? → [Answer]
5. Why did [4] happen? → [Root cause]

### Contributing Factors
- [Factor 1]
- [Factor 2]

### Action Items
| Action | Owner | Due Date | Status |
|--------|-------|----------|--------|
| [Action 1] | [Name] | YYYY-MM-DD | TODO |
| [Action 2] | [Name] | YYYY-MM-DD | TODO |

### Lessons Learned
- [Insight 1]
- [Insight 2]
```

---

## Error Budget Policy (BRD Section 15.2)

When error budget drops below 50% remaining:
1. **Feature freeze** — No non-reliability deployments
2. **All engineering focus on reliability improvements**
3. **Mandatory review of all recent changes**
4. **Increase chaos engineering frequency**

When error budget is exhausted (0% remaining):
1. **Full deployment freeze except hotfixes**
2. **Executive escalation**
3. **Dedicated reliability sprint until budget recovers**
