package policies

import rego.v1

# Deny by default (zero-trust security model)
default allow := false

# Allow any request bearing a validated JWT
allow if {
	input.jwt.aud
}

# Anonymous (no Authorization header) requests to /mcp are denied here, which
# triggers rest-rego's RFC 9728 401 + WWW-Authenticate response (see policy.go)
# instead of a plain 403 - since RESOURCE_URL is configured for this deployment.
