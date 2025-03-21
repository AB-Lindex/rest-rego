package policies

# This policy is used to control access to the API
default allow := false

allow if count(roles) > 0

# Assign roles
roles contains "admin" if {
	admin_apps := {
		"11112222-3333-4444-5555-666677778888",
		"33334444-5555-6666-7777-888899990000", # name-of-application
	}
	input.user.appId in admin_apps
}

roles contains "user" if {
	user_apps := {
		"11112222-3333-4444-5555-666677778888",
		"22223333-4444-5555-6666-777788889999",
		"33334444-5555-6666-7777-888899990000", # name-of-application
	}
	input.user.appId in user_apps
}

# Extract the app_id from the verified user
app_id := input.user.appId

# URL rewrite for better metrics
default url := ""

url := path if {
	input.request.path[0] == "user"
	path := "/user/--"
}
