
acl  = "write"
mesh = "write"

agent_prefix "" {
  policy = "read"
}
node "{{.Name}}" {
  policy = "write"
}
service_prefix "" {
  policy = "write"
}