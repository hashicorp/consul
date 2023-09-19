
node "{{.Name}}" {
	policy = "write"
}
service_prefix "" {
	policy = "read"
}