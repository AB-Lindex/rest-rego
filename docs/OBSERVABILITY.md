# Observability Guide

Comprehensive guide to monitoring, logging, and troubleshooting rest-rego in production.

## Table of Contents

- [Health Checks](#health-checks)
- [Prometheus Metrics](#prometheus-metrics)
- [Structured Logging](#structured-logging)
- [Alerting Recommendations](#alerting-recommendations)
- [Dashboards](#dashboards)
- [Debugging](#debugging)

## Health Checks

rest-rego provides Kubernetes-compatible health endpoints on the management port (default: 8182).

### Available Endpoints

| Endpoint | Purpose | Response | Use For |
|----------|---------|----------|---------|
| `/healthz` | Liveness probe | `200 OK` when service is running | K8s liveness probe |
| `/readyz` | Readiness probe | `200 OK` when policies loaded | K8s readiness probe, LB health |
| `/metrics` | Prometheus metrics | Metrics in Prometheus format | Prometheus scraping |

### Health Check Details

#### `/healthz` - Liveness Probe

Indicates the service is alive and not deadlocked.

**Response when healthy:**
```
200 OK
OK
```

**Response when unhealthy:**
```
503 Service Unavailable
Service Unavailable
```

**Use case**: Kubernetes liveness probe to restart unhealthy containers.

#### `/readyz` - Readiness Probe

Indicates the service is ready to accept traffic (policies loaded, auth configured).

**Response when ready:**
```
200 OK
OK
```

**Response when not ready:**
```
503 Service Unavailable
Not Ready
```

**Use case**: Kubernetes readiness probe to control traffic routing.

### Kubernetes Configuration

#### Liveness Probe

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8182
  initialDelaySeconds: 2
  periodSeconds: 10
  timeoutSeconds: 2
  failureThreshold: 3
```

**Recommendations:**
- `initialDelaySeconds`: 2-5 seconds (rest-rego starts fast)
- `periodSeconds`: 10 seconds (frequent checks)
- `failureThreshold`: 3 (restart after 30 seconds of failure)

#### Readiness Probe

```yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: 8182
  initialDelaySeconds: 2
  periodSeconds: 5
  timeoutSeconds: 2
  failureThreshold: 2
  successThreshold: 1
```

**Recommendations:**
- `initialDelaySeconds`: 2-5 seconds (policy loading is fast)
- `periodSeconds`: 5 seconds (more frequent than liveness)
- `failureThreshold`: 2 (remove from service after 10 seconds)

#### Combined Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: rest-rego
        image: lindex/rest-rego:latest
        ports:
        - containerPort: 8181
          name: http
        - containerPort: 8182
          name: metrics
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8182
          initialDelaySeconds: 2
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8182
          initialDelaySeconds: 2
          periodSeconds: 5
```

## Prometheus Metrics

rest-rego exports detailed metrics in Prometheus format on `/metrics` endpoint (port 8182).

### Core Metrics

#### Request Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `restrego_requests_total` | Counter | `method`, `url`, `result` | Total requests by method, path, result (allow/deny) |
| `restrego_request_duration_seconds` | Histogram | `method`, `url` | Request processing latency distribution |

**Example:**
```promql
# Request rate by result
rate(restrego_requests_total[5m])

# P99 latency
histogram_quantile(0.99, rate(restrego_request_duration_seconds_bucket[5m]))

# Deny rate
rate(restrego_requests_total{result="deny"}[5m])
```

#### Authentication Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `restrego_auth_total` | Counter | `method`, `result` | Authentication attempts by method and result |
| `restrego_jwk_cache_hits_total` | Counter | - | JWK cache hits (JWT mode only) |
| `restrego_jwk_cache_misses_total` | Counter | - | JWK cache misses (JWT mode only) |
| `restrego_graph_api_calls_total` | Counter | `result` | Microsoft Graph API calls (Azure mode only) |

**Example:**
```promql
# Authentication failure rate
rate(restrego_auth_total{result="failure"}[5m])

# JWK cache hit rate
rate(restrego_jwk_cache_hits_total[5m]) / 
  (rate(restrego_jwk_cache_hits_total[5m]) + rate(restrego_jwk_cache_misses_total[5m]))
```

#### Policy Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `restrego_policy_evaluation_seconds` | Histogram | - | Policy evaluation duration distribution |
| `restrego_policy_reload_total` | Counter | `result` | Policy reload events by result (success/failure) |

**Example:**
```promql
# Policy evaluation P99 latency
histogram_quantile(0.99, rate(restrego_policy_evaluation_seconds_bucket[5m]))

# Policy reload failures
rate(restrego_policy_reload_total{result="failure"}[5m])
```

#### Blocked Headers Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `restrego_blocked_headers_exposed` | Gauge | - | Feature state (0=disabled, 1=enabled) |
| `restrego_blocked_headers_captured` | Counter | - | Total number of blocked headers captured |
| `restrego_requests_with_blocked_headers` | Counter | - | Total requests containing blocked headers |

**Example:**
```promql
# Rate of requests with blocked headers
rate(restrego_requests_with_blocked_headers[5m])

# Average blocked headers per request
rate(restrego_blocked_headers_captured[5m]) / rate(restrego_requests_with_blocked_headers[5m])
```

### Prometheus Configuration

#### Scrape Config

```yaml
scrape_configs:
  - job_name: 'rest-rego'
    kubernetes_sd_configs:
    - role: pod
    relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app]
      action: keep
      regex: rest-rego
    - source_labels: [__meta_kubernetes_pod_container_port_name]
      action: keep
      regex: metrics
    - source_labels: [__meta_kubernetes_namespace]
      target_label: namespace
    - source_labels: [__meta_kubernetes_pod_name]
      target_label: pod
```

#### ServiceMonitor (Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: rest-rego
spec:
  selector:
    matchLabels:
      app: rest-rego
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

### Example Grafana Queries

#### Authorization Overview

```promql
# Total request rate
sum(rate(restrego_requests_total[5m]))

# Allow vs Deny rate
sum by (result) (rate(restrego_requests_total[5m]))

# Allow rate percentage
sum(rate(restrego_requests_total{result="allow"}[5m])) / 
  sum(rate(restrego_requests_total[5m])) * 100
```

#### Performance

```promql
# P50, P95, P99 latency
histogram_quantile(0.50, rate(restrego_request_duration_seconds_bucket[5m]))
histogram_quantile(0.95, rate(restrego_request_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(restrego_request_duration_seconds_bucket[5m]))

# Average latency
rate(restrego_request_duration_seconds_sum[5m]) / 
  rate(restrego_request_duration_seconds_count[5m])
```

#### Authentication

```promql
# Auth success rate
sum(rate(restrego_auth_total{result="success"}[5m])) / 
  sum(rate(restrego_auth_total[5m])) * 100

# JWK cache efficiency
rate(restrego_jwk_cache_hits_total[5m]) / 
  (rate(restrego_jwk_cache_hits_total[5m]) + rate(restrego_jwk_cache_misses_total[5m])) * 100
```

#### Policy Performance

```promql
# Policy evaluation latency P99
histogram_quantile(0.99, rate(restrego_policy_evaluation_seconds_bucket[5m]))

# Policy reload success rate
sum(rate(restrego_policy_reload_total{result="success"}[5m])) / 
  sum(rate(restrego_policy_reload_total[5m])) * 100
```

## Structured Logging

rest-rego uses structured JSON logging for easy parsing and integration with log aggregation systems.

### Log Levels

| Level | When Used | Examples |
|-------|-----------|----------|
| `INFO` | Normal operations | Startup, config, policy reload, request handling |
| `WARN` | Non-fatal issues | Policy validation warnings, cache misses |
| `ERROR` | Errors requiring attention | Auth failures, policy errors, backend errors |
| `DEBUG` | Detailed debugging | Policy input/output, request details (requires `--verbose`) |

### Log Format

```json
{
  "time": "2025-11-20T10:30:45Z",
  "level": "INFO",
  "msg": "policy evaluation completed",
  "request_id": "req-abc123",
  "method": "GET",
  "path": "/api/users",
  "result": "allow",
  "duration_ms": 2.3
}
```

### Common Log Messages

#### Startup

```json
{
  "time": "2025-11-20T10:00:00Z",
  "level": "INFO",
  "msg": "rest-rego starting",
  "version": "v1.2.3",
  "listen_addr": ":8181",
  "mgmt_addr": ":8182",
  "backend": "http://localhost:8080"
}
```

#### Policy Reload

```json
{
  "time": "2025-11-20T10:05:00Z",
  "level": "INFO",
  "msg": "policy reloaded",
  "files": 3,
  "duration_ms": 45.2
}
```

#### Request Allowed

```json
{
  "time": "2025-11-20T10:30:00Z",
  "level": "INFO",
  "msg": "request allowed",
  "method": "GET",
  "path": "/api/users",
  "appid": "11112222-3333-4444-5555-666677778888",
  "duration_ms": 3.1
}
```

#### Request Denied

```json
{
  "time": "2025-11-20T10:31:00Z",
  "level": "WARN",
  "msg": "request denied",
  "method": "POST",
  "path": "/api/admin",
  "appid": "22223333-4444-5555-6666-777788889999",
  "reason": "policy_denied"
}
```

#### Authentication Failure

```json
{
  "time": "2025-11-20T10:32:00Z",
  "level": "ERROR",
  "msg": "authentication failed",
  "method": "GET",
  "path": "/api/users",
  "error": "token expired"
}
```

### Debug Mode

Enable debug mode to see full policy input and output:

```bash
# Via flag
rest-rego --debug

# Via environment variable
DEBUG=true rest-rego
```

**Debug output example:**
```json
{
  "time": "2025-11-20T10:30:00Z",
  "level": "DEBUG",
  "msg": "policy evaluation",
  "input": {
    "request": {"method": "GET", "path": ["api", "users"]},
    "jwt": {"appid": "test-app", "roles": ["admin"]}
  },
  "output": {
    "allow": true
  },
  "duration_ms": 2.1
}
```

### Verbose Logging

Enable verbose logging for detailed debugging:

```bash
rest-rego --verbose
```

**Includes:**
- JWK cache operations
- Policy compilation details
- Backend connection events
- Header processing

### Log Aggregation

#### Fluentd Configuration

```yaml
<source>
  @type tail
  path /var/log/containers/rest-rego-*.log
  pos_file /var/log/fluentd-rest-rego.pos
  tag rest-rego
  <parse>
    @type json
    time_key time
    time_format %Y-%m-%dT%H:%M:%S%z
  </parse>
</source>

<filter rest-rego>
  @type record_transformer
  <record>
    service rest-rego
    environment ${ENV}
  </record>
</filter>

<match rest-rego>
  @type elasticsearch
  host elasticsearch
  port 9200
  index_name rest-rego-%Y.%m.%d
</match>
```

#### Promtail Configuration (Loki)

```yaml
scrape_configs:
- job_name: rest-rego
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_label_app]
    action: keep
    regex: rest-rego
  pipeline_stages:
  - json:
      expressions:
        level: level
        msg: msg
        method: method
        path: path
  - labels:
      level:
      method:
```

## Alerting Recommendations

### Critical Alerts

Fire immediately, require immediate action:

```yaml
# Service down
alert: RestRegoDown
expr: up{job="rest-rego"} == 0
for: 1m
severity: critical
annotations:
  summary: "rest-rego instance is down"

# High error rate
alert: RestRegoHighErrorRate
expr: rate(restrego_auth_total{result="failure"}[5m]) / rate(restrego_auth_total[5m]) > 0.5
for: 5m
severity: critical
annotations:
  summary: "rest-rego error rate above 50%"

# High latency
alert: RestRegoHighLatency
expr: histogram_quantile(0.99, rate(restrego_request_duration_seconds_bucket[5m])) > 0.1
for: 5m
severity: critical
annotations:
  summary: "rest-rego P99 latency above 100ms"

# Policy reload failures
alert: RestRegoPolicyReloadFailing
expr: rate(restrego_policy_reload_total{result="failure"}[5m]) > 0
for: 1m
severity: critical
annotations:
  summary: "rest-rego policy reload failures detected"
```

### Warning Alerts

May indicate issues, monitor closely:

```yaml
# Moderate error rate
alert: RestRegoModerateErrorRate
expr: rate(restrego_auth_total{result="failure"}[5m]) / rate(restrego_auth_total[5m]) > 0.05
for: 10m
severity: warning
annotations:
  summary: "rest-rego error rate above 5%"

# Moderate latency
alert: RestRegoModerateLatency
expr: histogram_quantile(0.99, rate(restrego_request_duration_seconds_bucket[5m])) > 0.05
for: 10m
severity: warning
annotations:
  summary: "rest-rego P99 latency above 50ms"

# Low JWK cache hit rate
alert: RestRegoLowJWKCacheHitRate
expr: rate(restrego_jwk_cache_hits_total[5m]) / (rate(restrego_jwk_cache_hits_total[5m]) + rate(restrego_jwk_cache_misses_total[5m])) < 0.95
for: 15m
severity: warning
annotations:
  summary: "rest-rego JWK cache hit rate below 95%"

# High deny rate (potential attack)
alert: RestRegoHighDenyRate
expr: rate(restrego_requests_total{result="deny"}[5m]) / rate(restrego_requests_total[5m]) > 0.3
for: 10m
severity: warning
annotations:
  summary: "rest-rego deny rate above 30% (potential attack)"
```

### Info Notifications

Informational, good to know:

```yaml
# Policy reload success
alert: RestRegoPolicyReloaded
expr: changes(restrego_policy_reload_total{result="success"}[5m]) > 0
for: 1m
severity: info
annotations:
  summary: "rest-rego policies reloaded successfully"

# Scaling event detected
alert: RestRegoScalingEvent
expr: changes(up{job="rest-rego"}[5m]) > 0
for: 1m
severity: info
annotations:
  summary: "rest-rego instance count changed"
```

## Dashboards

### Grafana Dashboard Example

Key panels to include:

1. **Request Rate** (Graph)
   - Query: `sum(rate(restrego_requests_total[5m]))`
   - Split by `result` (allow/deny)

2. **Latency** (Graph)
   - P50: `histogram_quantile(0.50, rate(restrego_request_duration_seconds_bucket[5m]))`
   - P95: `histogram_quantile(0.95, rate(restrego_request_duration_seconds_bucket[5m]))`
   - P99: `histogram_quantile(0.99, rate(restrego_request_duration_seconds_bucket[5m]))`

3. **Authentication Success Rate** (Gauge)
   - Query: `sum(rate(restrego_auth_total{result="success"}[5m])) / sum(rate(restrego_auth_total[5m])) * 100`

4. **Policy Evaluation Latency** (Graph)
   - Query: `histogram_quantile(0.99, rate(restrego_policy_evaluation_seconds_bucket[5m]))`

5. **Top Denied Paths** (Table)
   - Query: `topk(10, sum by (url) (rate(restrego_requests_total{result="deny"}[5m])))`

6. **JWK Cache Hit Rate** (Gauge)
   - Query: `rate(restrego_jwk_cache_hits_total[5m]) / (rate(restrego_jwk_cache_hits_total[5m]) + rate(restrego_jwk_cache_misses_total[5m])) * 100`

## Debugging

### Enable Debug Mode

```bash
# Full debug output
rest-rego --debug --verbose

# Docker
docker run -e DEBUG=true -e VERBOSE=true lindex/rest-rego:latest

# Kubernetes
kubectl set env deployment/rest-rego DEBUG=true VERBOSE=true
```

### Viewing Logs

```bash
# Docker
docker logs -f rest-rego

# Kubernetes
kubectl logs -f deployment/rest-rego -c rest-rego

# Filter for errors
kubectl logs deployment/rest-rego -c rest-rego | grep -i error

# Watch for policy reloads
kubectl logs -f deployment/rest-rego -c rest-rego | grep -i reload
```

### Testing Metrics Endpoint

```bash
# Local
curl http://localhost:8182/metrics

# Kubernetes port-forward
kubectl port-forward deployment/rest-rego 8182:8182
curl http://localhost:8182/metrics

# Check specific metric
curl -s http://localhost:8182/metrics | grep restrego_requests_total
```

### Common Debugging Scenarios

#### High Latency

1. Check policy evaluation time: `restrego_policy_evaluation_seconds`
2. Check backend response time: `restrego_request_duration_seconds` - `restrego_policy_evaluation_seconds`
3. Review policy complexity
4. Profile policy with OPA: `opa eval --profile`

#### Authentication Failures

1. Enable verbose logging
2. Check JWK cache: `restrego_jwk_cache_misses_total`
3. Verify OIDC well-known endpoint accessibility
4. Test JWT token with jwt.io
5. Review `restrego_auth_total{result="failure"}` by method

#### Policy Reload Failures

1. Check logs for syntax errors
2. Validate policy files: `opa check policies/`
3. Review `restrego_policy_reload_total{result="failure"}`
4. Test policy locally with sample input

## Related Documentation

- [Configuration Reference](./CONFIGURATION.md) - All configuration options
- [Troubleshooting Guide](./TROUBLESHOOTING.md) - Common issues and solutions
- [Deployment Guide](./DEPLOYMENT.md) - Production deployment patterns
- [Policy Development Guide](./POLICY.md) - Writing and testing policies
