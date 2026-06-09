package policies

default allow := false

# Allow requests that carry a valid JWT with a non-empty subject.
# The policy intentionally accesses several JWT fields so any per-eval
# allocation growth shows up clearly in heap profiles.
allow if {
	input.jwt.sub != ""
	input.jwt.aud[_] == "jwt-e2e-test"
}
