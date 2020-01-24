module github.com/hashicorp/consul/api

go 1.12

replace github.com/hashicorp/consul/sdk => ../sdk

require (
	github.com/fatih/color v1.9.0 // indirect
	github.com/hashicorp/consul/sdk v0.2.0
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-hclog v0.12.0 // indirect
	github.com/hashicorp/go-rootcerts v1.0.0
	github.com/hashicorp/go-uuid v1.0.1
	github.com/hashicorp/serf v0.8.2
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mitchellh/mapstructure v1.1.2
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.4.0
	golang.org/x/sys v0.0.0-20200124204421-9fbb57f87de9 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
)
