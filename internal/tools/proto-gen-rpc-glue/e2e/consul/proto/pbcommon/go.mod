module github.com/hashicorp/consul/proto/pbcommon

go 1.26.9

require (
	github.com/hashicorp/consul/agent/structs v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.11
)

replace github.com/hashicorp/consul/agent/structs => ../../agent/structs
