package request.rego

import rego.v1

default allow := false

allow if {
    input.request.auth.kind == "basic"
    input.request.auth.user != ""
}

allow if { input.request.path[0] == "public" }

# Assign custom header forwarded to backend
user := input.request.auth.user