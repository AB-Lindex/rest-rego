package policies

import rego.v1

# Deny by default (zero-trust security model)
default allow := false

# Allow any request from a known, authenticated user
allow if {
	input.request.auth.kind == "basic"
	input.request.auth.user != ""
}

# Allow public endpoints without authentication
allow if {
	input.request.path[0] == "public"
}

# Restrict the /admin path to the alice account only
allow if {
	input.request.auth.kind == "basic"
	input.request.auth.user == "alice"
	input.request.path[0] == "admin"
}
