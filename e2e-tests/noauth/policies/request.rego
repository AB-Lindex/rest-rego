package policies

default allow := false

allow if {
	endswith(input.request.path[count(input.request.path)-1], "-allow")
}
