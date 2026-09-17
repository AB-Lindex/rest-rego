package policies

import rego.v1

# Deny by default (zero-trust security model)
default allow := false
default labels := {}

allow if {
	input.jwt.appid == "$(ALLOWED_APP_ID)"
}

# Expose the client version header as a custom Prometheus metric label.
# Requires METRIC_LABELS=client_version to register the label at startup.
labels := {"client_version": v} if {
	v := input.request.headers["X-Client-Version"]
}
