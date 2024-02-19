module github.com/hashicorp/consul/troubleshoot

go 1.19

replace (
	github.com/hashicorp/consul/api => ../api
	github.com/hashicorp/consul/envoyextensions => ../envoyextensions
)

exclude (
	github.com/hashicorp/go-msgpack v1.1.5 // has breaking changes and must be avoided
	github.com/hashicorp/go-msgpack v1.1.6 // contains retractions but same as v1.1.5
)

require (
	github.com/envoyproxy/go-control-plane v0.11.1
	github.com/envoyproxy/go-control-plane/xdsmatcher v0.0.0-20230524161521-aaaacbfbe53e
	github.com/hashicorp/consul/api v1.26.1
	github.com/hashicorp/consul/envoyextensions v0.5.1
	github.com/hashicorp/consul/sdk v0.15.0
	github.com/stretchr/testify v1.8.3
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cncf/xds/go v0.0.0-20230607035331-e9ce68804cb4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.1 // indirect
	github.com/fatih/color v1.14.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.2.1 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	go.opentelemetry.io/proto/otlp v0.19.0 // indirect
	golang.org/x/exp v0.0.0-20230817173708-d852ddb80c63 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto v0.0.0-20230526203410-71b5a4ffd15e // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230526203410-71b5a4ffd15e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230526203410-71b5a4ffd15e // indirect
	google.golang.org/grpc v1.56.3 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
