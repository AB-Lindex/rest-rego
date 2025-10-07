# Why Use rest-rego?

## The Problem: Authorization is Hard to Get Right

You've built a REST API. Now you need to secure it. The typical path looks like this:

1. Add JWT validation to your service
2. Parse tokens, verify signatures, check expiration
3. Write authorization logic: "Can this user do this action?"
4. Repeat this in every service you build
5. Debug subtle security bugs in production
6. Update authorization logic across 10+ services when requirements change

**There's a better way.**

## The Solution: Centralized Authorization as a Sidecar

rest-rego is a specialized authorization proxy that sits between your API and the internet. It handles authentication and authorization so your application doesn't have to.

```
Client → rest-rego (auth + policies) → Your API (business logic only)
```

## Why Developers Choose rest-rego

### 1. **Ship Faster: Focus on Business Logic**

**Without rest-rego:**
```
// Your API code becomes cluttered with auth logic
function HandleOrder(request, response) {
    // Parse JWT
    token = extractToken(request)
    if (token is empty) {
        return Unauthorized(401, "Unauthorized")
    }
    
    // Verify signature
    claims = verifyJWT(token, publicKey)
    if (claims is invalid) {
        return Unauthorized(401, "Invalid token")
    }
    
    // Check expiration
    if (currentTime > claims.expiration) {
        return Unauthorized(401, "Token expired")
    }
    
    // Authorization logic
    if (!hasRole(claims, "order-manager")) {
        return Forbidden(403, "Forbidden")
    }
    
    // Finally, your actual business logic
    processOrder(...)
}
```

**With rest-rego:**
```
// Your API code stays clean
function HandleOrder(request, response) {
    // rest-rego already validated auth
    // Just do your business logic
    processOrder(...)
}
```

**Result**: Write 70% less code. Deploy features faster.

### 2. **Security by Design: Fail Closed, Not Open**

| Your Implementation | rest-rego |
|---------------------|-----------|
| Easy to forget auth checks | Deny-by-default - nothing passes without explicit policy |
| Auth logic scattered across codebase | Centralized policies - audit in one place |
| Hard to test security thoroughly | OPA testing framework - TDD for authorization |
| Production bugs = security incidents | Validated in seconds with hot reload |

**Real scenario**: A developer forgets to add auth to a new endpoint. With custom code, this means an open security hole. With rest-rego, the endpoint is denied by default until a policy explicitly allows it.

### 3. **Policy-as-Code: Authorization You Can Read**

Stop writing spaghetti if-statements. Write policies that express intent:

```rego
# Simple, readable authorization policy
package policies

default allow = false

# Order managers can manage orders
allow {
    input.request.path[0] == "orders"
    "order-manager" in input.jwt.roles
}

# Admins can do anything
allow {
    "admin" in input.jwt.roles
}
```

**Benefits:**
- **Readable**: Non-developers can review policies
- **Versioned**: Policies live in Git alongside your code
- **Testable**: Use OPA's built-in testing framework
- **Auditable**: Complete history of authorization changes

### 4. **Performance That Doesn't Hurt**

"But won't a proxy slow everything down?"

**No.** rest-rego adds **< 5ms latency** (p99) while handling **5,000+ requests/second** per instance.

| Operation | Latency |
|-----------|---------|
| JWT verification (cached) | < 1ms |
| Policy evaluation | < 3ms |
| Total overhead | < 5ms |

**Why so fast?**
- Written in Go (compiled, concurrent)
- JWK keys cached locally with automatic refresh from OIDC endpoint
- Optimized middleware pipeline
- No external API calls in hot path (JWT mode)

**Key security feature**: JWK keys are automatically refreshed from the OIDC provider's well-known endpoint, ensuring your service always has the latest signing keys without manual intervention. When identity providers rotate keys, rest-rego picks up the changes seamlessly.

**Works with any OIDC provider**: Cloud-based (Azure AD, Auth0, Okta) or on-premises (WSO2, Keycloak, self-hosted solutions). As long as it's standards-compliant OIDC, rest-rego works.

Compare this to:
- Database query for permissions: 10-50ms
- External auth API call: 50-200ms
- Your hand-rolled auth code: Probably slower than you think

### 5. **Zero Code Changes to Your Application**

Deploy rest-rego as a sidecar container. Your application needs **zero changes**:

```yaml
# Kubernetes deployment
spec:
  containers:
  - name: your-app
    image: your-app:latest
    ports:
    - containerPort: 8080
    
  - name: rest-rego
    image: lindex/rest-rego:latest
    env:
    - name: BACKEND_PORT
      value: "8080"  # Forward to your app
    - name: WELLKNOWN_OIDC
      value: "https://your-idp.com/.well-known/openid-configuration"
    - name: JWT_AUDIENCES
      value: "your-api-audience"  # Required: Expected audience claim
```

**Traffic flow:** Internet → rest-rego:8181 → localhost:8080 (your app)

Your app doesn't even know rest-rego exists.

### 6. **Hot Policy Updates: No Restart Required**

Authorization requirements change. A lot.

**Traditional approach:** 
1. Change code
2. Build new image
3. Deploy to production
4. 10-minute deployment cycle

**rest-rego approach:**
1. Edit `.rego` file
2. Save
3. **Policy reloads in < 1 second**

**Real story**: A customer needs emergency access to a resource. With rest-rego, you edit a policy and save. Access granted in 1 second. With code changes, you're looking at a 10-minute minimum deployment cycle (or emergency hotfix process).

### 7. **Consistency Across Services**

Building microservices? You need consistent authorization everywhere.

**Problem with DIY auth:**
- Service A: Custom JWT validation
- Service B: Different JWT library, subtly different validation
- Service C: Someone copy-pasted old code with a bug
- Service D: New developer implements it differently

**With rest-rego:**
- Same authorization logic everywhere
- Update once, applies to all services
- Single source of truth for policies
- Consistent audit logs

### 8. **Production-Grade Observability**

Get instant visibility into authorization decisions:

```prometheus
# Prometheus metrics out of the box
restrego_requests_total{method="GET",path="/orders",result="allow"} 1523
restrego_requests_total{method="GET",path="/orders",result="deny"} 47
restrego_request_duration_seconds_bucket{le="0.005"} 1489
restrego_auth_total{method="jwt",result="success"} 1570
```

**Build dashboards that show:**
- Which applications access which endpoints
- Authorization denial patterns (possible attacks?)
- Performance impact of authorization layer
- Policy reload success/failure rates

**Debugging:** Enable debug mode to see exact policy input/output for any request. No more "why was this denied?" mysteries.

## The Honest Cons

Let's be real about tradeoffs:

| Consideration | Impact | Mitigation |
|---------------|--------|------------|
| **Additional component** | One more thing to deploy | Tiny container (< 50MB), starts in < 3 seconds |
| **Learning Rego** | New policy language | Simpler than you think, extensive examples provided |
| **Network hop** | Extra proxy in path | < 5ms overhead, negligible for most APIs |
| **Debugging policies** | Different mental model | Debug mode shows exact policy evaluation |
| **Policy testing** | Need to test policies | OPA testing framework makes this straightforward |

**When NOT to use rest-rego:**
- **Ultra-low latency requirements**: If you need < 1ms response times, every millisecond counts
- **Simple public APIs**: If your API is fully public with no auth, you don't need this
- **Non-REST protocols**: rest-rego is HTTP-only (no gRPC, WebSocket, etc.)
- **Database-level authorization**: This protects HTTP APIs, not database queries

## Comparison: DIY vs rest-rego

### Building Your Own JWT Validation

| Aspect | Your Implementation | rest-rego |
|--------|---------------------|-----------|
| **Initial development** | 2-5 days per service | 30 minutes deployment |
| **JWT signature verification** | Pick library, implement, test | Built-in with JWK caching & auto-refresh |
| **Token expiration** | Manual checks, edge cases | Automatic validation |
| **OIDC provider support** | Parse well-known endpoints | Auto-discovers from OIDC URL |
| **Key rotation handling** | Manual refresh logic, monitoring | Automatic JWK refresh, zero downtime |
| **Provider flexibility** | Vendor lock-in risk | Works with any OIDC provider (cloud or on-premises) |
| **Authorization logic** | if/else in every handler | Declarative Rego policies |
| **Policy updates** | Code change + deployment | Edit file, 1-second reload |
| **Testing** | Write unit tests for auth | OPA testing framework + your unit tests |
| **Audit trail** | Custom logging | Structured logs + Prometheus metrics |
| **Multi-service consistency** | Copy-paste (prone to drift) | Deploy once, same everywhere |
| **Security bugs** | Your responsibility to find | Battle-tested OPA engine |
| **Maintenance burden** | Ongoing per-service maintenance | Central component, single point of maintenance |

### Using a Heavy API Gateway

| Aspect | Kong/Apigee/Tyk | rest-rego |
|--------|-----------------|-----------|
| **Complexity** | Full API gateway with 100+ features | Focused on authorization only |
| **Resource usage** | 200-500MB+ per instance | 50-100MB per instance |
| **Latency** | 10-50ms overhead | < 5ms overhead |
| **Learning curve** | Steep (gateway concepts) | Moderate (Rego policies) |
| **Cost** | Enterprise licenses ($$$) | Open source (free) |
| **Flexibility** | Opinionated workflows | Policy-as-code - full control |
| **Deployment** | Complex setup | Single container sidecar |

## Real-World Scenarios

### Scenario 1: "We need to add authorization to 15 microservices"

**DIY approach:**
- 2-3 days per service = 30-45 developer days
- Inconsistent implementations
- Maintenance burden across 15 codebases

**rest-rego approach:**
- Deploy sidecar to all 15 services = 2-3 days total
- Write policies once, apply everywhere
- Single component to maintain

**Savings: 90% less development time**

### Scenario 2: "Our authorization rules change weekly"

**Code-based auth:**
- Each change requires code deployment
- 10-30 minutes per deployment
- Risk of bugs with each change

**rest-rego:**
- Edit policy file
- 1-second reload
- No deployment, no risk of breaking changes

**Result: 10x faster iteration**

### Scenario 3: "We need to audit who accessed what"

**Custom implementation:**
- Add logging to every endpoint
- Parse logs for audit reports
- Inconsistent log formats

**rest-rego:**
- Built-in structured logging
- Prometheus metrics by endpoint/user
- Complete audit trail automatically

**Result: Compliance-ready from day one**

## Getting Started: Try It in 5 Minutes

Convinced? Here's how to try it:

```bash
# 1. Pull the image
docker pull lindex/rest-rego:latest

# 2. Create a simple policy (allow everything)
echo 'package policies
default allow = true' > request.rego

# 3. Run it
docker run -p 8181:8181 \
  -v $(pwd):/policies \
  -e BACKEND_PORT=your-api-port \
  -e WELLKNOWN_OIDC=https://your-idp/.well-known/openid-configuration \
  -e JWT_AUDIENCES=your-api-audience \
  lindex/rest-rego

# 4. Test it
curl -H "Authorization: Bearer YOUR_JWT" http://localhost:8181/your-endpoint
```

See authorization working in less time than it takes to read JWT library documentation.

## The Bottom Line

**Use rest-rego if:**
- ✅ You're building REST APIs that need authorization
- ✅ You want to focus on business logic, not security boilerplate
- ✅ You value consistency across services
- ✅ You need policy updates faster than code deployments
- ✅ You want production-grade observability

**Stick with DIY if:**
- ❌ You have ultra-low latency requirements (< 1ms)
- ❌ Your authorization logic is extremely simple (single public/private flag)
- ❌ You have unlimited development time and love writing auth code
- ❌ You don't need to update authorization logic frequently

## Next Steps

1. **Read the README**: Quick start guide and basic configuration
2. **Review the PRD**: Comprehensive technical specifications in `/.specs/PRD.md`
3. **Try the examples**: Kubernetes deployment examples in `/examples/`
4. **Write a policy**: Start with simple rules and expand
5. **Deploy as sidecar**: Zero-code-change deployment

**Questions?** Check out:
- 📖 [README.md](./README.md) - Quick start guide
- 📋 [PRD.md](./.specs/PRD.md) - Complete product requirements
- 🔐 [JWT Authentication](./docs/JWT.md) - JWT configuration guide
- ☁️ [Azure Authentication](./docs/AZURE.md) - Azure Graph setup
- 🚀 [Examples](./examples/) - Kubernetes deployment examples

---

**TL;DR**: rest-rego lets you ship features faster, secure your APIs properly, and sleep better at night knowing authorization is handled by a battle-tested, policy-driven sidecar instead of hand-rolled code scattered across your codebase.
