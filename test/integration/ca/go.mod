module github.com/hashicorp/consul/integration/ca

go 1.16

require (
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/docker/docker v20.10.11+incompatible
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/hashicorp/consul/api v1.11.0
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/memberlist v0.3.1 // indirect
	github.com/hashicorp/serf v0.9.6 // indirect
	github.com/hashicorp/vault/api v1.4.1
	github.com/mitchellh/go-testing-interface v1.14.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.12.0
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
)
replace github.com/hashicorp/consul/api => ../../../api

replace github.com/hashicorp/consul/sdk => ../../../sdk