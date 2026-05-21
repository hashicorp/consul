module github.com/hashicorp/consul/internal/tools/protoc-gen-consul-rate-limit

go 1.26

replace github.com/hashicorp/consul/proto-public/v2 => ../../../proto-public

require (
	github.com/hashicorp/consul/proto-public/v2 v2.0.0
	google.golang.org/protobuf v1.36.11
)
