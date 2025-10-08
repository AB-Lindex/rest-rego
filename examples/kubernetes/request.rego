package policies

# Deny by default (zero-trust security model)
default allow := false

# Allow specific applications by JWT appid claim
allow if {
	valid_apps := {
		"11112222-3333-4444-5555-666677778888", # name-of-application-1
		"22223333-4444-5555-6666-777788889999", # name-of-application-2
		"33334444-5555-6666-7777-888899990000", # name-of-application-3
	}
	input.jwt.appid in valid_apps
}

# Allow users with admin role
allow if {
	"admin" in input.jwt.roles
}

# Allow public endpoints without authentication
allow if {
	input.request.path[0] == "public"
}

# Optional: URL rewriting example
default url := ""

url := path if {
	input.request.path[0] == "user"
	path := "/user/--" # redact user/tenant IDs from metrics paths
}
