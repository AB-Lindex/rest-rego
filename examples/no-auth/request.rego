package policies

import rego.v1

# Read-only methods are allowed without any additional credentials
read_only_methods := ["GET", "HEAD", "OPTIONS"]

# Deny by default (zero-trust security model)
# input.jwt and input.user are null in no-auth mode — policies must not rely on them.
default allow := false

allow if {
	input.request.method in read_only_methods
}

# Mutating methods require a shared secret via the X-Api-Key header
allow if {
	not input.request.method in read_only_methods
	input.request.headers["X-Api-Key"] == "$(EXPECTED_API_KEY)"
}
