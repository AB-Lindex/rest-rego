package policies

# Deny by default (zero-trust security model)
default allow := false

# Allow all requests with valid JWT signatures
# This policy validates that:
# - The JWT was successfully parsed and verified by rest-rego
# - The JWT has required standard claims (issuer, subject, expiration)
allow if {
	input.jwt.iss  # Issuer claim exists
	input.jwt.sub  # Subject claim exists
	input.jwt.exp  # Expiration claim exists
}

# Allow public health check endpoints without authentication
# These are typically used by load balancers and monitoring systems
allow if {
	input.request.path[0] == "health"
}

allow if {
	input.request.path[0] == "ready"
}

# Allow unauthenticated access to public paths
allow if {
	input.request.path[0] == "public"
}

# Example: Role-based access control
# Uncomment to require specific roles in the JWT
# allow if {
#     "admin" in input.jwt.roles
# }

# Example: Audience validation (already handled by JWT_AUDIENCES env var)
# Additional audience checks can be done here if needed
# allow if {
#     input.jwt.aud == "https://example.com"
# }

# Optional: URL rewriting example
default url := ""

# Example: Redact sensitive path segments from upstream requests
# url := "/api/redacted" if {
#     input.request.path[0] == "api"
#     input.request.path[1] == "sensitive"
# }
