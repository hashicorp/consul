agent "{{.Name}}" {
  policy = "read"
}
node "{{.Name}}" {
  policy = "write"
}
service_prefix "" {
  policy = "write"
}
