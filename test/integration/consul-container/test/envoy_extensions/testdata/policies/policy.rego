package envoy.authz

import future.keywords

import input.attributes.request.http as http_request

default allow := false

allow if {
	http_request.method == "GET"
	glob.match("/allow", ["/"], http_request.path)
}
