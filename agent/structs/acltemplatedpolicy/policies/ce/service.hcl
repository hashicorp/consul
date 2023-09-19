
service "{{.Name}}" {
	policy = "write"
}
service "{{.Name}}-sidecar-proxy" {
	policy = "write"
}
service_prefix "" {
	policy = "read"
}
node_prefix "" {
	policy = "read"
}