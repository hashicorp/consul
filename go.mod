module github.com/hashicorp/consul

go 1.12

replace github.com/hashicorp/consul/api => ./api

replace github.com/hashicorp/consul/sdk => ./sdk

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Azure/go-autorest v10.15.3+incompatible // indirect
	github.com/Jeffail/gabs v1.1.0 // indirect
	github.com/Microsoft/go-winio v0.4.3 // indirect
	github.com/NYTimes/gziphandler v1.0.1
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/SAP/go-hdb v0.12.0 // indirect
	github.com/SermoDigital/jose v0.0.0-20180104203859-803625baeddc // indirect
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6 // indirect
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878
	github.com/armon/go-radix v1.0.0
	github.com/asaskevich/govalidator v0.0.0-20180319081651-7d2e70ef918f // indirect
	github.com/bitly/go-hostpool v0.0.0-20171023180738-a3a6125de932 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible // indirect
	github.com/containerd/continuity v0.0.0-20181203112020-004b46473808 // indirect
	github.com/coredns/coredns v1.1.2
	github.com/denisenkom/go-mssqldb v0.0.0-20180620032804-94c9c97e8c9f // indirect
	github.com/digitalocean/godo v1.10.0 // indirect
	github.com/docker/go-connections v0.3.0
	github.com/docker/go-units v0.3.3 // indirect
	github.com/duosecurity/duo_api_golang v0.0.0-20190308151101-6c680f768e74 // indirect
	github.com/elazarl/go-bindata-assetfs v0.0.0-20160803192304-e1a2a7ec64b0
	github.com/envoyproxy/go-control-plane v0.8.0
	github.com/fatih/structs v0.0.0-20180123065059-ebf56d35bba7 // indirect
	github.com/go-ldap/ldap v3.0.2+incompatible // indirect
	github.com/go-ole/go-ole v1.2.1 // indirect
	github.com/go-sql-driver/mysql v0.0.0-20180618115901-749ddf1598b4 // indirect
	github.com/gocql/gocql v0.0.0-20180617115710-e06f8c1bcd78 // indirect
	github.com/gogo/googleapis v1.1.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.1
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db // indirect
	github.com/google/go-github v17.0.0+incompatible // indirect
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/hashicorp/consul/api v1.2.0
	github.com/hashicorp/consul/sdk v0.2.0
	github.com/hashicorp/go-bexpr v0.1.2
	github.com/hashicorp/go-checkpoint v0.0.0-20171009173528-1545e56e46de
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-discover v0.0.0-20190403160810-22221edb15cd
	github.com/hashicorp/go-hclog v0.9.1
	github.com/hashicorp/go-memdb v0.0.0-20180223233045-1289e7fffe71
	github.com/hashicorp/go-msgpack v0.5.5
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/go-plugin v0.0.0-20180331002553-e8d22c780116
	github.com/hashicorp/go-raftchunking v0.6.1
	github.com/hashicorp/go-sockaddr v1.0.0
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/go-uuid v1.0.1
	github.com/hashicorp/go-version v0.0.0-20170202080759-03c5bf6be031
	github.com/hashicorp/golang-lru v0.5.0
	github.com/hashicorp/hcl v0.0.0-20180906183839-65a6292f0157
	github.com/hashicorp/hil v0.0.0-20160711231837-1e86c6b523c5
	github.com/hashicorp/logutils v1.0.0
	github.com/hashicorp/mdns v1.0.1 // indirect
	github.com/hashicorp/memberlist v0.1.5
	github.com/hashicorp/net-rpc-msgpackrpc v0.0.0-20151116020338-a14192a58a69
	github.com/hashicorp/raft v1.1.1
	github.com/hashicorp/raft-boltdb v0.0.0-20171010151810-6e5ba93211ea
	github.com/hashicorp/serf v0.8.2
	github.com/hashicorp/vault v0.10.3
	github.com/hashicorp/vault-plugin-secrets-kv v0.0.0-20190318174639-195e0e9d07f1 // indirect
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d
	github.com/imdario/mergo v0.3.6
	github.com/jefferai/jsonx v0.0.0-20160721235117-9cc31c3135ee // indirect
	github.com/keybase/go-crypto v0.0.0-20180614160407-5114a9a81e1b // indirect
	github.com/kr/text v0.1.0
	github.com/lib/pq v0.0.0-20180523175426-90697d60dd84 // indirect
	github.com/miekg/dns v1.0.14
	github.com/mitchellh/cli v1.0.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/mitchellh/go-testing-interface v1.0.0
	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452
	github.com/mitchellh/mapstructure v1.1.2
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/oklog/run v0.0.0-20180308005104-6934b124db28 // indirect
	github.com/onsi/gomega v1.4.2 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/ory/dockertest v3.3.4+incompatible // indirect
	github.com/pascaldekloe/goe v0.1.0
	github.com/patrickmn/go-cache v0.0.0-20180527043350-9f6ff22cfff8 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.2
	github.com/ryanuber/columnize v0.0.0-20160712163229-9b3edd62028f
	github.com/ryanuber/go-glob v0.0.0-20170128012129-256dc444b735 // indirect
	github.com/shirou/gopsutil v0.0.0-20181107111621-48177ef5f880
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/stretchr/objx v0.1.1 // indirect
	github.com/stretchr/testify v1.3.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/net v0.0.0-20190503192946-f4e77d36d62c
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	google.golang.org/grpc v1.23.0
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/mgo.v2 v2.0.0-20160818020120-3f83fa500528 // indirect
	gopkg.in/ory-am/dockertest.v3 v3.3.4 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/api v0.0.0-20190325185214-7544f9db76f6
	k8s.io/apimachinery v0.0.0-20190223001710-c182ff3b9841
	k8s.io/client-go v8.0.0+incompatible
)
