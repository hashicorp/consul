module github.com/hashicorp/consul/api

go 1.12

replace github.com/hashicorp/consul/sdk => ../sdk

require (
	github.com/hashicorp/consul/sdk v0.2.0
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-hclog v0.12.0
	github.com/hashicorp/go-rootcerts v1.0.2
	github.com/hashicorp/go-uuid v1.0.1
	github.com/hashicorp/serf v0.8.2
	github.com/kr/pretty v0.2.0 // indirect
	github.com/mitchellh/mapstructure v1.1.2
	github.com/stretchr/testify v1.4.0
)
