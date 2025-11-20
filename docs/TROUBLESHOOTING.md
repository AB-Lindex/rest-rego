# Troubleshooting Guide

Common issues and solutions for rest-rego deployment and operation.

## Table of Contents

- [Authentication Issues](#authentication-issues)
- [Policy Evaluation Issues](#policy-evaluation-issues)
- [Policy Reload Issues](#policy-reload-issues)
- [Performance Issues](#performance-issues)
- [Connection Issues](#connection-issues)
- [Configuration Issues](#configuration-issues)
- [Deployment Issues](#deployment-issues)
- [Getting Help](#getting-help)

## Authentication Issues

### 401 Unauthorized Responses

**Symptoms:**
- All requests return 401 Unauthorized
- Logs show "authentication failed" errors

**Common Causes & Solutions:**

#### 1. Expired or Invalid JWT Token

```bash
# Verify token is valid and not expired
# Use jwt.io or CLI tool
echo $TOKEN | jwt decode -

# Check expiration time
echo $TOKEN | jwt decode - | jq '.exp'
```

**Solution:** Obtain a fresh token from your identity provider.

#### 2. Audience Mismatch

```bash
# Check token audience
echo $TOKEN | jwt decode - | jq '.aud'

# Compare with configured audience
echo $JWT_AUDIENCES
```

**Solution:** Ensure `JWT_AUDIENCES` matches the `aud` claim in your token.

#### 3. OIDC Well-Known URL Unreachable

```bash
# Test OIDC endpoint accessibility
curl -v https://login.microsoftonline.com/TENANT/v2.0/.well-known/openid-configuration

# From rest-rego container
docker exec rest-rego wget -O- $WELLKNOWN_OIDC
```

**Solution:** 
- Verify URL is correct
- Check network connectivity
- Verify firewall rules allow outbound HTTPS

#### 4. JWK Cache Not Initialized

**Symptoms:** Logs show "jwks not loaded" or "key not found in jwks"

```bash
# Check logs for JWK loading
docker logs rest-rego | grep -i jwk

# Look for successful load message
# "loaded jwks from well-known endpoint"
```

**Solution:**
- Wait for initial JWK load (happens on startup)
- Verify OIDC well-known endpoint returns valid JWKS URL
- Check network connectivity to JWKS endpoint

### 403 Forbidden with Valid Authentication

**Symptoms:**
- Authentication succeeds (token valid)
- Request still denied with 403

**Cause:** Policy evaluation returned `allow = false`

**Solution:** See [Policy Evaluation Issues](#policy-evaluation-issues)

### WSO2-Specific Issues

#### Custom Audience Claim Not Found

```bash
# Check if custom claim exists in token
echo $TOKEN | jwt decode - | jq '."http://wso2.org/claims/apiname"'
```

**Solution:**
```bash
export JWT_AUDIENCE_KEY="http://wso2.org/claims/apiname"
export JWT_AUDIENCES="YourAPIName"
```

#### Wrong Authentication Header

**Solution:**
```bash
export AUTH_HEADER="X-Jwt-Assertion"
export AUTH_KIND=""  # Empty to skip Bearer prefix check
```

## Policy Evaluation Issues

### 403 Forbidden When Access Should Be Allowed

**Symptoms:**
- Valid authentication
- Request denied by policy
- Should be allowed based on policy rules

**Debug Steps:**

#### 1. Enable Debug Mode

```bash
# See full policy input and output
rest-rego --debug

# Docker
docker run -e DEBUG=true lindex/rest-rego:latest

# Kubernetes
kubectl set env deployment/my-app DEBUG=true -c rest-rego
```

#### 2. Review Policy Input

Debug output shows what the policy receives:

```json
{
  "input": {
    "request": {
      "method": "GET",
      "path": ["api", "users"],
      "headers": {...}
    },
    "jwt": {
      "appid": "...",
      "roles": [...]
    }
  },
  "output": {
    "allow": false
  }
}
```

**Check:**
- Is `input.jwt.appid` what you expect?
- Are `input.jwt.roles` correct?
- Is `input.request.path` structured as expected?

#### 3. Test Policy Locally

```bash
# Create test input file
cat > input.json <<EOF
{
  "request": {
    "method": "GET",
    "path": ["api", "users"]
  },
  "jwt": {
    "appid": "test-app-id",
    "roles": ["admin"]
  }
}
EOF

# Test with OPA CLI
opa eval -d policies/ -I 'data.policies.allow' < input.json

# Should output: true
```

#### 4. Validate Policy Syntax

```bash
# Check for syntax errors
opa check policies/

# Run policy tests (if you have test files)
opa test policies/ -v
```

**Common Policy Mistakes:**

```rego
# ❌ Wrong: Using == for array membership
allow if {
  input.jwt.appid == ["app1", "app2"]
}

# ✅ Correct: Use 'in' operator
allow if {
  input.jwt.appid in ["app1", "app2"]
}

# ❌ Wrong: Forgetting to check if claim exists
allow if {
  "admin" in input.jwt.roles
}

# ✅ Correct: Handle missing claims
allow if {
  count(input.jwt.roles) > 0
  "admin" in input.jwt.roles
}

# ❌ Wrong: Wrong package name
package request

# ✅ Correct: Must be 'policies'
package policies
```

### Policy Always Returns False

**Symptoms:**
- All requests denied
- Debug shows `allow: false` for all inputs

**Check:**

#### 1. Default Allow Rule Present

```rego
# Required in every policy
default allow := false
```

#### 2. At Least One Allow Rule

```rego
# Must have at least one rule that can succeed
allow if {
  # Some condition
}
```

#### 3. Allow Rules Are Reachable

```bash
# Test with minimal input
echo '{}' | opa eval -d policies/ -I 'data.policies.allow'

# If returns false, no allow rules are reachable with empty input
# This is usually correct (deny by default)
```

## Policy Reload Issues

### Policy Changes Don't Take Effect

**Symptoms:**
- Policy file edited and saved
- Behavior doesn't change
- No reload events in logs

**Debug Steps:**

#### 1. Check Logs for Reload Events

```bash
# Look for reload messages
docker logs -f rest-rego | grep -i reload

# Successful reload:
# "policy reloaded successfully"

# Failed reload:
# "policy reload failed"
```

#### 2. Verify File Permissions

```bash
# Policies must be readable
ls -la policies/

# Should show read permissions (r--)
# -r--r--r-- 1 user group 1234 Nov 20 10:00 request.rego
```

#### 3. Check File Pattern Matches

```bash
# Default pattern is *.rego
ls policies/*.rego

# If using custom pattern
export FILE_PATTERN="*.policy"
ls policies/*.policy
```

#### 4. Validate Policy Syntax

```bash
# Invalid policies are rejected silently
opa check policies/

# Fix any syntax errors before saving
```

#### 5. Kubernetes ConfigMap Updates

```bash
# Check if ConfigMap updated
kubectl get configmap my-app-policies -o yaml

# ConfigMap changes take time to propagate (up to 60s)
# For immediate update, restart pods
kubectl rollout restart deployment/my-app
```

### Policy Reload Failures

**Symptoms:**
- Logs show "policy reload failed"
- `restrego_policy_reload_total{result="failure"}` metric increases

**Common Causes:**

#### 1. Syntax Errors

```bash
# Validate before deploying
opa check policies/

# Example error:
# 1 error occurred: request.rego:5: rego_parse_error: unexpected eof token
```

**Solution:** Fix syntax errors before saving.

#### 2. Package Name Mismatch

```rego
# ❌ Wrong
package request

# ✅ Correct
package policies
```

#### 3. Duplicate Rule Definitions

```rego
# ❌ Wrong: Same rule defined twice
allow if { input.jwt.appid == "app1" }
allow if { input.jwt.appid == "app2" }

# ✅ Correct: Use single rule with OR
allow if {
  input.jwt.appid in ["app1", "app2"]
}
```

## Performance Issues

### High Latency

**Symptoms:**
- `restrego_request_duration_seconds` > 50ms
- Slow API responses
- Users complaining about performance

**Debug Steps:**

#### 1. Check Metrics Breakdown

```bash
# Get metrics
curl http://localhost:8182/metrics

# Check policy evaluation time
curl -s http://localhost:8182/metrics | grep restrego_policy_evaluation_seconds

# Check overall request time
curl -s http://localhost:8182/metrics | grep restrego_request_duration_seconds
```

**Analysis:**
- If policy evaluation > 5ms: Policy is too complex
- If (total - policy) > 20ms: Backend is slow

#### 2. Profile Policy Performance

```bash
# Create sample input
cat > input.json <<EOF
{
  "request": {"method": "GET", "path": ["api", "users"]},
  "jwt": {"appid": "test", "roles": ["admin"]}
}
EOF

# Profile policy evaluation
opa eval -d policies/ --profile -I 'data.policies.allow' < input.json
```

**Output shows:**
```
+---------+----------+
| Rule    | Time     |
+---------+----------+
| allow   | 234.5µs  |
| helper  | 12.3µs   |
+---------+----------+
```

**Optimization Tips:**

```rego
# ❌ Slow: Nested loops
allow if {
  some role in input.jwt.roles
  some app in valid_apps
  app == input.jwt.appid
}

# ✅ Fast: Use sets and membership tests
valid_apps := {"app1", "app2", "app3"}

allow if {
  input.jwt.appid in valid_apps
  "admin" in input.jwt.roles
}
```

#### 3. Check Backend Performance

```bash
# Test backend directly (bypass rest-rego)
time curl http://backend:8080/api/endpoint

# If slow, issue is with backend, not rest-rego
```

#### 4. Review Resource Allocation

```bash
# Kubernetes: Check CPU throttling
kubectl top pod -l app=my-app

# Increase CPU if throttled
kubectl set resources deployment/my-app \
  --requests=cpu=500m --limits=cpu=1000m \
  -c rest-rego
```

### Low Throughput

**Symptoms:**
- Cannot handle expected request volume
- CPU at 100%
- Requests timing out

**Solutions:**

#### 1. Horizontal Scaling

```bash
# Kubernetes
kubectl scale deployment/my-app --replicas=5

# Or use HPA
kubectl autoscale deployment/my-app --min=3 --max=10 --cpu-percent=70
```

#### 2. Optimize Policies

- Simplify complex rules
- Use sets instead of arrays
- Avoid nested loops
- Cache computed values

#### 3. Increase Resources

```yaml
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 2000m
    memory: 1Gi
```

## Connection Issues

### Cannot Connect to Backend

**Symptoms:**
- 502 Bad Gateway errors
- Logs show "dial tcp: connection refused"
- `restrego_backend_connection_errors_total` metric increases

**Debug Steps:**

#### 1. Verify Backend Configuration

```bash
# Check environment variables
docker exec rest-rego env | grep BACKEND

# Should see:
# BACKEND_SCHEME=http
# BACKEND_HOST=localhost
# BACKEND_PORT=8080
```

#### 2. Test Backend Connectivity

```bash
# From rest-rego container
docker exec rest-rego wget -O- http://localhost:8080/health

# If fails, backend is not reachable
```

**Common Causes:**

#### Sidecar Pattern

```yaml
# ✅ Correct: Both containers share localhost
containers:
- name: app
  ports:
  - containerPort: 8080
- name: rest-rego
  env:
  - name: BACKEND_HOST
    value: "localhost"  # Same pod
  - name: BACKEND_PORT
    value: "8080"
```

#### Separate Services

```yaml
# ✅ Correct: Use service name
env:
- name: BACKEND_HOST
  value: "backend-service"  # Service name
- name: BACKEND_PORT
  value: "80"  # Service port
```

#### Docker Compose

```yaml
# ✅ Correct: Use service name
services:
  rest-rego:
    environment:
      BACKEND_HOST: "backend"  # Service name
```

#### 3. Check Network Policies

```bash
# Verify network policy allows traffic
kubectl describe networkpolicy -n production

# Test connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  wget -O- http://backend:8080/health
```

### Connection Timeouts

**Symptoms:**
- Requests timeout after 30s
- Logs show "context deadline exceeded"

**Solutions:**

#### 1. Increase Timeouts

```bash
# For slow backends
export READ_TIMEOUT="60s"
export WRITE_TIMEOUT="120s"
export BACKEND_RESPONSE_TIMEOUT="60s"
```

#### 2. Check Backend Health

```bash
# Backend might be slow or hanging
curl -w "@curl-format.txt" http://backend:8080/api/endpoint

# curl-format.txt:
# time_namelookup: %{time_namelookup}s
# time_connect: %{time_connect}s
# time_starttransfer: %{time_starttransfer}s
# time_total: %{time_total}s
```

## Configuration Issues

### Conflicting Authentication Configuration

**Error:**
```
cannot use both Azure Tenant and OIDC well-known endpoints
```

**Solution:** Choose one authentication method:

```bash
# JWT authentication
export WELLKNOWN_OIDC="https://..."
export JWT_AUDIENCES="..."
unset AZURE_TENANT

# OR Azure Graph authentication
export AZURE_TENANT="..."
unset WELLKNOWN_OIDC
unset JWT_AUDIENCES
```

### Missing Required Configuration

**Error:**
```
JWT audiences required when using OIDC well-known endpoints
```

**Solution:**
```bash
export WELLKNOWN_OIDC="https://..."
export JWT_AUDIENCES="api://your-audience"  # Required!
```

### Invalid Timeout Values

**Error:**
```
read timeout must be between 1s and 10m, got: 15m
```

**Solution:**
```bash
# Use valid timeout (1s to 10m)
export READ_TIMEOUT="5m"  # Not 15m
```

## Deployment Issues

### Pods Not Starting (Kubernetes)

**Symptoms:**
- Pods in CrashLoopBackOff
- ImagePullBackOff errors

**Debug:**

```bash
# Check pod status
kubectl get pods -l app=my-app

# Check events
kubectl describe pod my-app-xxx

# Check logs
kubectl logs my-app-xxx -c rest-rego
```

**Common Causes:**

#### 1. Image Pull Errors

```bash
# Check image exists and is accessible
docker pull lindex/rest-rego:latest

# Check image pull secrets if using private registry
kubectl get secret regcred -n production
```

#### 2. Configuration Errors

```bash
# Check ConfigMap exists
kubectl get configmap my-app-policies

# Check ConfigMap content
kubectl get configmap my-app-policies -o yaml
```

#### 3. Resource Limits

```bash
# Check if OOMKilled
kubectl get pod my-app-xxx -o jsonpath='{.status.containerStatuses[*].lastState}'

# Increase memory if needed
kubectl set resources deployment/my-app \
  --limits=memory=512Mi -c rest-rego
```

### Health Checks Failing

**Symptoms:**
- Pods not reaching Ready state
- Constant restarts

**Debug:**

```bash
# Check health endpoint
kubectl port-forward my-app-xxx 8182:8182
curl http://localhost:8182/healthz
curl http://localhost:8182/readyz
```

**Solutions:**

#### 1. Increase Initial Delay

```yaml
readinessProbe:
  initialDelaySeconds: 10  # Was: 2
```

#### 2. Check Policy Loading

```bash
# Logs should show successful policy load
kubectl logs my-app-xxx -c rest-rego | grep -i policy

# Look for: "policies loaded successfully"
```

## Getting Help

### Enable Verbose Logging

```bash
rest-rego --verbose --debug
```

### Collect Diagnostics

```bash
# Configuration
env | grep -E '(WELLKNOWN|JWT|AZURE|BACKEND|POLICY)'

# Metrics snapshot
curl http://localhost:8182/metrics > metrics.txt

# Recent logs
docker logs rest-rego --tail 1000 > logs.txt

# Policy files
tar -czf policies.tar.gz policies/
```

### Resources

- 📖 **Documentation**: [docs/](../docs/) folder
- 💬 **GitHub Issues**: [github.com/AB-Lindex/rest-rego/issues](https://github.com/AB-Lindex/rest-rego/issues)
- 🔐 **Security Issues**: See [SECURITY.md](../SECURITY.md)
- 📋 **Examples**: [examples/](../examples/) folder
- 🧪 **Tests**: [tests/](../tests/) folder

### Issue Template

When reporting issues, include:

1. **Environment:**
   - rest-rego version: `rest-rego --version`
   - Deployment platform: Docker / Kubernetes / etc
   - Authentication mode: JWT / Azure Graph

2. **Configuration:**
   - Relevant environment variables (redact secrets!)
   - Policy files (if applicable)

3. **Problem:**
   - Expected behavior
   - Actual behavior
   - Error messages
   - Logs (with `--verbose --debug`)

4. **Reproduction:**
   - Steps to reproduce
   - Sample request that fails
   - Policy that should allow it

## Related Documentation

- [Configuration Reference](./CONFIGURATION.md) - All configuration options
- [Policy Development](./POLICY.md) - Writing and testing policies
- [Observability](./OBSERVABILITY.md) - Monitoring and debugging
- [Deployment Guide](./DEPLOYMENT.md) - Production deployment
