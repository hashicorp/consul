module github.com/hashicorp/consul

go 1.20

replace (
	github.com/hashicorp/consul/api => ./api
	github.com/hashicorp/consul/envoyextensions => ./envoyextensions
	github.com/hashicorp/consul/proto-public => ./proto-public
	github.com/hashicorp/consul/sdk => ./sdk
	github.com/hashicorp/consul/troubleshoot => ./troubleshoot
)

exclude (
	github.com/hashicorp/go-msgpack v1.1.5 // has breaking changes and must be avoided
	github.com/hashicorp/go-msgpack v1.1.6 // contains retractions but same as v1.1.5
)

require (
	github.com/NYTimes/gziphandler v1.0.1
	github.com/aliyun/alibaba-cloud-sdk-go v1.62.156
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/armon/go-metrics v0.4.1
	github.com/armon/go-radix v1.0.0
	github.com/aws/aws-sdk-go v1.44.289
	github.com/coreos/go-oidc/v3 v3.9.0
	github.com/deckarep/golang-set/v2 v2.3.1
	github.com/docker/go-connections v0.4.0
	github.com/envoyproxy/go-control-plane v0.12.0
	github.com/envoyproxy/go-control-plane/xdsmatcher v0.0.0-20230524161521-aaaacbfbe53e
	github.com/fatih/color v1.16.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/fullstorydev/grpchan v1.1.1
	github.com/go-jose/go-jose/v3 v3.0.3
	github.com/go-openapi/runtime v0.26.2
	github.com/go-openapi/strfmt v0.21.10
	github.com/google/go-cmp v0.5.9
	github.com/google/gofuzz v1.2.0
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1
	github.com/google/tcpproxy v0.0.0-20180808230851-dfa16c61dad2
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/hashi-derek/grpc-proxy v0.0.0-20231207191910-191266484d75
	github.com/hashicorp/consul-awsauth v0.0.0-20220713182709-05ac1c5c2706
	github.com/hashicorp/consul-net-rpc v0.0.0-20221205195236-156cfab66a69
	github.com/hashicorp/consul/api v1.26.1
	github.com/hashicorp/consul/envoyextensions v0.5.1
	github.com/hashicorp/consul/proto-public v0.5.1
	github.com/hashicorp/consul/sdk v0.15.0
	github.com/hashicorp/consul/troubleshoot v0.4.1
	github.com/hashicorp/go-bexpr v0.1.2
	github.com/hashicorp/go-checkpoint v0.5.0
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/hashicorp/go-connlimit v0.3.0
	github.com/hashicorp/go-discover v0.0.0-20230724184603-e89ebd1b2f65
	github.com/hashicorp/go-hclog v1.5.0
	github.com/hashicorp/go-immutable-radix v1.3.1
	github.com/hashicorp/go-immutable-radix/v2 v2.1.0
	github.com/hashicorp/go-memdb v1.3.4
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-raftchunking v0.7.0
	github.com/hashicorp/go-retryablehttp v0.6.7
	github.com/hashicorp/go-rootcerts v1.0.2
	github.com/hashicorp/go-secure-stdlib/awsutil v0.1.6
	github.com/hashicorp/go-sockaddr v1.0.2
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/go-uuid v1.0.3
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hashicorp/hcdiag v0.5.1
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hcl/v2 v2.14.1
	github.com/hashicorp/hcp-scada-provider v0.2.4
	github.com/hashicorp/hcp-sdk-go v0.80.0
	github.com/hashicorp/hil v0.0.0-20200423225030-a18a1cd20038
	github.com/hashicorp/memberlist v0.5.0
	github.com/hashicorp/raft v1.5.0
	github.com/hashicorp/raft-autopilot v0.1.6
	github.com/hashicorp/raft-boltdb/v2 v2.2.2
	github.com/hashicorp/raft-wal v0.4.1
	github.com/hashicorp/serf v0.10.1
	github.com/hashicorp/vault-plugin-auth-alicloud v0.14.0
	github.com/hashicorp/vault/api v1.12.2
	github.com/hashicorp/vault/api/auth/gcp v0.3.0
	github.com/hashicorp/vault/sdk v0.7.0
	github.com/hashicorp/yamux v0.0.0-20211028200310-0bc27b27de87
	github.com/imdario/mergo v0.3.15
	github.com/kr/text v0.2.0
	github.com/miekg/dns v1.1.50
	github.com/mitchellh/cli v1.1.4
	github.com/mitchellh/copystructure v1.2.0
	github.com/mitchellh/go-testing-interface v1.14.0
	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/mitchellh/mapstructure v1.5.0
	github.com/mitchellh/pointerstructure v1.2.1
	github.com/mitchellh/reflectwalk v1.0.2
	github.com/natefinch/npipe v0.0.0-20160621034901-c1b8fa8bdcce
	github.com/oklog/ulid v1.3.1
	github.com/oklog/ulid/v2 v2.1.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.14.0
	github.com/rboyer/safeio v0.2.3
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/shirou/gopsutil/v3 v3.22.9
	github.com/stretchr/testify v1.8.4
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/zclconf/go-cty v1.11.1
	go.etcd.io/bbolt v1.3.7
	go.opentelemetry.io/otel v1.17.0
	go.opentelemetry.io/otel/metric v1.17.0
	go.opentelemetry.io/otel/sdk v1.17.0
	go.opentelemetry.io/otel/sdk/metric v0.39.0
	go.opentelemetry.io/proto/otlp v1.0.0
	go.uber.org/goleak v1.1.10
	golang.org/x/crypto v0.22.0
	golang.org/x/exp v0.0.0-20230817173708-d852ddb80c63
	golang.org/x/net v0.24.0
	golang.org/x/oauth2 v0.15.0
	golang.org/x/sync v0.3.0
	golang.org/x/sys v0.19.0
	golang.org/x/time v0.3.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98
	google.golang.org/grpc v1.58.3
	google.golang.org/protobuf v1.33.0
	gotest.tools/v3 v3.4.0
	k8s.io/api v0.26.2
	k8s.io/apimachinery v0.26.2
	k8s.io/client-go v0.26.2
)

require (
	cloud.google.com/go/compute v1.21.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v1.1.1 // indirect
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.28 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.18 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.12 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.5 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/DataDog/datadog-go v4.8.2+incompatible // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/Masterminds/sprig/v3 v3.2.2 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v13 v13.0.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/benbjohnson/immutable v0.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/cenkalti/backoff/v3 v3.0.0 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/circonus-labs/circonus-gometrics v2.3.1+incompatible // indirect
	github.com/circonus-labs/circonusllhist v0.1.3 // indirect
	github.com/cncf/xds/go v0.0.0-20230607035331-e9ce68804cb4 // indirect
	github.com/coreos/etcd v3.3.27+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/pkg v0.0.0-20220810130054-c7d1c02cb6cf // indirect
	github.com/cosiner/argv v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/denverdino/aliyungo v0.0.0-20170926055100-d3308649c661 // indirect
	github.com/digitalocean/godo v1.10.0 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/analysis v0.21.5 // indirect
	github.com/go-openapi/errors v0.21.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.1 // indirect
	github.com/go-openapi/jsonreference v0.20.3 // indirect
	github.com/go-openapi/loads v0.21.3 // indirect
	github.com/go-openapi/spec v0.20.12 // indirect
	github.com/go-openapi/swag v0.22.5 // indirect
	github.com/go-openapi/validate v0.22.4 // indirect
	github.com/go-ozzo/ozzo-validation v3.6.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/s2a-go v0.1.4 // indirect
	github.com/google/uuid v1.4.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/googleapis/gax-go/v2 v2.11.0 // indirect
	github.com/gophercloud/gophercloud v0.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.0.0 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.6 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.0 // indirect
	github.com/hashicorp/mdns v1.0.4 // indirect
	github.com/hashicorp/net-rpc-msgpackrpc/v2 v2.0.0 // indirect
	github.com/hashicorp/vic v1.5.1-0.20190403131502-bbfe86ec9443 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/jhump/protoreflect v1.11.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/joyent/triton-go v1.7.1-0.20200416154420-6801d15b779f // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/linode/linodego v0.10.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nicolai86/scaleway-sdk v1.10.2-0.20180628010248-798f60e20bb2 // indirect
	github.com/opentracing/opentracing-go v1.2.1-0.20220228012449-10b1cf09e00b // indirect
	github.com/packethost/packngo v0.1.1-0.20180711074735-b9cb5096f54c // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/posener/complete v1.2.3 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/renier/xmlrpc v0.0.0-20170708154548-ce4a1a486c03 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966 // indirect
	github.com/softlayer/softlayer-go v0.0.0-20180806151055-260589d94c7d // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go v1.0.162 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.5.0 // indirect
	github.com/tv42/httpunix v0.0.0-20150427012821-b75d8614f926 // indirect
	github.com/vmware/govmomi v0.18.0 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.mongodb.org/mongo-driver v1.13.1 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel/trace v1.17.0 // indirect
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/term v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.12.1-0.20230815132531-74c255bcf846 // indirect
	google.golang.org/api v0.126.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20230711160842-782d3b101e98 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230711160842-782d3b101e98 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.66.2 // indirect
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce // indirect
	gopkg.in/resty.v1 v1.12.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280 // indirect
	k8s.io/utils v0.0.0-20230406110748-d93618cff8a2 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
