module github.com/hashicorp/consul/api

go 1.12

replace github.com/hashicorp/consul/sdk => ../sdk

require (
	github.com/google/go-cmp v0.5.7
	github.com/hashicorp/consul/sdk v0.13.0
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-hclog v0.12.0
	github.com/hashicorp/go-rootcerts v1.0.2
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.2
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/serf v0.10.1
	github.com/kr/pretty v0.2.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.1
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)
