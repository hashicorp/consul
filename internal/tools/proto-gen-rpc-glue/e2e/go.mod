module github.com/hashicorp/consul/internal/tools/proto-gen-rpc-glue/e2e

go 1.26.10

require (
	github.com/hashicorp/consul/agent/structs v0.0.0-00010101000000-000000000000
	github.com/hashicorp/consul/proto/pbcommon v0.0.0-00010101000000-000000000000
)

require google.golang.org/protobuf v1.36.11 // indirect

replace github.com/hashicorp/consul/proto/pbcommon => ./consul/proto/pbcommon

replace github.com/hashicorp/consul/agent/structs => ./consul/agent/structs
