module github.com/hashicorp/consul/internal/tools/proto-gen-rpc-glue/e2e

go 1.13

replace github.com/hashicorp/consul => ../../../..

replace github.com/hashicorp/consul/api => ../../../../api

replace github.com/hashicorp/consul/sdk => ../../../../sdk

require github.com/hashicorp/consul v1.11.4
