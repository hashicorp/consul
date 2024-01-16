mesh = "read"
node_prefix "" {
	policy = "read"
}
service_prefix "" {
	policy = "read"
}
service "{{.Name}}" {
	policy = "write"
}