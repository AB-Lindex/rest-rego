package policies

default allow := false

allow if {
	print("appid-check", input.user.appId)
	valid_apps := {
		"11112222-3333-4444-5555-666677778888", # name-of-application-1
		"22223333-4444-5555-6666-777788889999", # name-of-application-2
		"33334444-5555-6666-7777-888899990000", # name-of-application-3
	}
	input.user.appId in valid_apps
}

allow if {
	input.request.path[0] == "public"
}

default url := ""

url := path if {
	input.request.path[0] == "user"
	path := "/user/--"
}

var3 := input.request.blocked_headers["X-Restrego-Var1"]

