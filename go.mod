module github.com/hashicorp/consul

go 1.12

replace github.com/hashicorp/consul/api => ./api

replace github.com/hashicorp/consul/sdk => ./sdk

require (
	github.com/Azure/go-autorest v10.15.3+incompatible // indirect
	github.com/Microsoft/go-winio v0.4.3 // indirect
	github.com/NYTimes/gziphandler v1.0.1
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6 // indirect
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878
	github.com/armon/go-radix v1.0.0
	github.com/coredns/coredns v1.1.2
	github.com/digitalocean/godo v1.10.0 // indirect
	github.com/docker/go-connections v0.3.0
	github.com/elazarl/go-bindata-assetfs v1.0.0
	github.com/envoyproxy/go-control-plane v0.8.0
	github.com/go-ole/go-ole v1.2.1 // indirect
	github.com/gogo/googleapis v1.1.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.2
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf
	github.com/hashicorp/consul/api v1.2.0
	github.com/hashicorp/consul/sdk v0.2.0
	github.com/hashicorp/go-bexpr v0.1.2
	github.com/hashicorp/go-checkpoint v0.0.0-20171009173528-1545e56e46de
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-discover v0.0.0-20190403160810-22221edb15cd
	github.com/hashicorp/go-hclog v0.9.2
	github.com/hashicorp/go-memdb v1.0.3
	github.com/hashicorp/go-msgpack v0.5.5
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/go-plugin v1.0.1
	github.com/hashicorp/go-raftchunking v0.6.1
	github.com/hashicorp/go-sockaddr v1.0.2
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/go-uuid v1.0.1
	github.com/hashicorp/go-version v1.1.0
	github.com/hashicorp/golang-lru v0.5.1
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hil v0.0.0-20160711231837-1e86c6b523c5
	github.com/hashicorp/logutils v1.0.0
	github.com/hashicorp/mdns v1.0.1 // indirect
	github.com/hashicorp/memberlist v0.1.5
	github.com/hashicorp/net-rpc-msgpackrpc v0.0.0-20151116020338-a14192a58a69
	github.com/hashicorp/raft v1.1.1
	github.com/hashicorp/raft-boltdb v0.0.0-20171010151810-6e5ba93211ea
	github.com/hashicorp/serf v0.8.5
	github.com/hashicorp/vault/api v1.0.4
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d
	github.com/imdario/mergo v0.3.6
	github.com/kr/text v0.1.0
	github.com/miekg/dns v1.0.14
	github.com/mitchellh/cli v1.0.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/mitchellh/go-testing-interface v1.0.0
	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452
	github.com/mitchellh/mapstructure v1.1.2
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/onsi/gomega v1.4.2 // indirect
	github.com/pascaldekloe/goe v0.1.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.2
	github.com/ryanuber/columnize v2.1.0+incompatible
	github.com/shirou/gopsutil v0.0.0-20181107111621-48177ef5f880
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/stretchr/objx v0.1.1 // indirect
	github.com/stretchr/testify v1.3.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	google.golang.org/grpc v1.23.0
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1
	k8s.io/api v0.0.0-20190325185214-7544f9db76f6
	k8s.io/apimachinery v0.0.0-20190223001710-c182ff3b9841
	k8s.io/client-go v8.0.0+incompatible
)
