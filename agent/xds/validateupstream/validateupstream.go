package validateupstream

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/builtinextensions/validate"
	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	// Copied from z_xds_packages
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/http/squash/v3"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/http/sxg/v3alpha"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/network/kafka_broker/v3"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/network/kafka_mesh/v3alpha"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/network/mysql_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/network/postgres_proxy/v3alpha"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/network/rocketmq_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/network/sip_proxy/router/v3alpha"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/filters/network/sip_proxy/v3alpha"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/private_key_providers/cryptomb/v3alpha"
	_ "github.com/envoyproxy/go-control-plane/contrib/envoy/extensions/vcl/v3alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/admin/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/annotations"
	_ "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	_ "github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	_ "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	_ "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	_ "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	_ "github.com/envoyproxy/go-control-plane/envoy/api/v2/ratelimit"
	_ "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/cluster/aggregate/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/cluster/dynamic_forward_proxy/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/cluster/redis"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/common/dynamic_forward_proxy/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/common/key_value/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/common/matcher/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/common/tap/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/dubbo/router/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/fault/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/adaptive_concurrency/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/aws_lambda/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/aws_request_signing/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/buffer/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/cache/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/compressor/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/cors/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/csrf/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/dynamic_forward_proxy/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/dynamo/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/ext_authz/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/fault/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/grpc_http1_bridge/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/grpc_http1_reverse_bridge/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/grpc_stats/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/grpc_web/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/gzip/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/header_to_metadata/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/health_check/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/ip_tagging/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/jwt_authn/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/lua/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/on_demand/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/original_src/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rate_limit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rbac/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/router/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/squash/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/tap/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/transcoder/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/http_inspector/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/original_dst/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/original_src/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/proxy_protocol/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/tls_inspector/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/client_ssl_auth/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/direct_response/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/dubbo_proxy/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/echo/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/ext_authz/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/kafka_broker/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/local_rate_limit/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/mongo_proxy/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/mysql_proxy/v1alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rate_limit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rbac/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/redis_proxy/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/sni_cluster/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/thrift_proxy/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/zookeeper_proxy/v1alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/thrift/rate_limit/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/thrift/router/v2alpha1"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/udp/udp_proxy/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/grpc_credential/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/grpc_credential/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/health_checker/redis/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/listener/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/metrics/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/metrics/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/overload/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/overload/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/ratelimit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/ratelimit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/resource_monitor/fixed_heap/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/resource_monitor/injected_resource/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/retry/omit_canary_hosts/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/retry/omit_host_metadata/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/retry/previous_hosts/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/retry/previous_priorities"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/tap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/trace/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/trace/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/trace/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/transport_socket/alts/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/transport_socket/raw_buffer/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/transport_socket/tap/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/cluster/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/cluster/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/core/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/core/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/dns/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/dns/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/tap/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/data/tap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/grpc/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/open_telemetry/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/stream/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/wasm/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/cache/simple_http_cache/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/aggregate/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/dynamic_forward_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/redis/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/common/dynamic_forward_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/common/matching/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/common/ratelimit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/common/tap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/compression/brotli/compressor/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/compression/brotli/decompressor/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/compression/gzip/compressor/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/compression/gzip/decompressor/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/common/dependency/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/common/fault/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/common/matcher/action/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/adaptive_concurrency/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/admission_control/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/alternate_protocols_cache/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/aws_lambda/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/aws_request_signing/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/bandwidth_limit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/buffer/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/cache/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/cdn_loop/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/composite/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/compressor/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/cors/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/csrf/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/decompressor/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/dynamic_forward_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/dynamo/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/fault/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_http1_bridge/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_http1_reverse_bridge/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_json_transcoder/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_stats/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_web/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/gzip/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/header_to_metadata/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/health_check/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ip_tagging/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/kill_request/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/oauth2/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/on_demand/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/original_src/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ratelimit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/set_metadata/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/tap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/http_inspector/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/original_dst/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/original_src/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/proxy_protocol/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/client_ssl_auth/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/connection_limit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/direct_response/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/dubbo_proxy/router/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/dubbo_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/echo/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/ext_authz/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/local_ratelimit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/mongo_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/ratelimit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/rbac/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/redis_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/sni_cluster/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/sni_dynamic_forward_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/thrift_proxy/filters/header_to_metadata/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/thrift_proxy/filters/ratelimit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/thrift_proxy/router/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/thrift_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/wasm/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/zookeeper_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/dns_filter/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/udp_proxy/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/formatter/metadata/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/formatter/req_without_query/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/health_checkers/redis/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/http/header_formatters/preserve_case/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/http/original_ip_detection/custom_header/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/http/original_ip_detection/xff/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/internal_redirect/allow_listed_routes/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/internal_redirect/previous_routes/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/internal_redirect/safe_cross_scheme/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/key_value/file_based/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/matching/common_inputs/environment_variable/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/matching/input_matchers/consistent_hashing/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/matching/input_matchers/ip/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/network/dns_resolver/apple/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/network/dns_resolver/cares/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/network/socket_interface/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/quic/crypto_stream/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/quic/proof_source/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/rate_limit_descriptors/expr/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/rbac/matchers/upstream_ip_port/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/request_id/uuid/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/resource_monitors/fixed_heap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/resource_monitors/injected_resource/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/retry/host/omit_canary_hosts/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/retry/host/omit_host_metadata/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/retry/host/previous_hosts/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/retry/priority/previous_priorities/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/stat_sinks/graphite_statsd/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/stat_sinks/wasm/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/alts/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/proxy_protocol/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/quic/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/raw_buffer/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/s2a/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/starttls/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tcp_stats/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/generic/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/http/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/tcp/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/tcp/generic/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/watchdog/profile_action/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/event_reporting/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/event_reporting/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/extension/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/health/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/metrics/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/metrics/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/status/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/status/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/tap/v2alpha"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/tap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/trace/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/service/trace/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/type"
	_ "github.com/envoyproxy/go-control-plane/envoy/type/http/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/type/matcher"
	_ "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/type/metadata/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/type/metadata/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/type/tracing/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/type/tracing/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/watchdog/v3"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	listenersType string = "type.googleapis.com/envoy.admin.v3.ListenersConfigDump"
	clustersType  string = "type.googleapis.com/envoy.admin.v3.ClustersConfigDump"
	routesType    string = "type.googleapis.com/envoy.admin.v3.RoutesConfigDump"
	endpointsType string = "type.googleapis.com/envoy.admin.v3.EndpointsConfigDump"
)

func ParseConfigDump(rawConfig []byte) (*xdscommon.IndexedResources, error) {
	config := &envoy_admin_v3.ConfigDump{}

	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err := unmarshal.Unmarshal(rawConfig, config)
	if err != nil {
		return nil, err
	}

	return proxyConfigDumpToIndexedResources(config)
}

func ParseClusters(rawClusters []byte) (*envoy_admin_v3.Clusters, error) {
	clusters := &envoy_admin_v3.Clusters{}
	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err := unmarshal.Unmarshal(rawClusters, clusters)
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

// Validate validates the Envoy resources (indexedResources) for a given upstream service, peer, and vip. The peer
// should be "" for an upstream not on a remote peer. The vip is required for a transparent proxy upstream.
func Validate(indexedResources *xdscommon.IndexedResources, clusters *envoy_admin_v3.Clusters, service api.CompoundServiceName, peer string, vip string) error {
	em := acl.NewEnterpriseMetaWithPartition(service.Partition, service.Namespace)
	svc := structs.ServiceName{Name: service.Name, EnterpriseMeta: em}

	// The envoyID is used to identify which listener and filter matches the upstream service.
	var envoyID string
	if peer != "" {
		psn := structs.PeeredServiceName{
			ServiceName: svc,
			Peer:        peer,
		}
		uid := proxycfg.NewUpstreamIDFromPeeredServiceName(psn)
		envoyID = uid.EnvoyID()
	} else {
		uid := proxycfg.NewUpstreamIDFromServiceName(svc)
		envoyID = uid.EnvoyID()
	}

	// Get all SNIs from the clusters in the configuration. Not all SNIs will need to be validated, but this ensures we
	// capture SNIs which aren't directly identical to the upstream service name, but are still used for that upstream
	// service. For example, in the case of having a splitter/redirect or another L7 config entry, the upstream service
	// name could be "db" but due to a redirect SNI would be something like
	// "redis.default.dc1.internal.<trustdomain>.consul". The envoyID will be used to limit which SNIs we actually
	// validate.
	snis := map[string]struct{}{}
	for s := range indexedResources.Index[xdscommon.ClusterType] {
		snis[s] = struct{}{}
	}

	// Build an ExtensionConfiguration for Validate plugin.
	extConfig := xdscommon.ExtensionConfiguration{
		EnvoyExtension: api.EnvoyExtension{
			Name: "builtin/proxy/validate",
			Arguments: map[string]interface{}{
				"envoyID": envoyID,
			},
		},
		ServiceName: service,
		Upstreams: map[api.CompoundServiceName]xdscommon.UpstreamData{
			service: {
				VIP:     vip,
				SNI:     snis,
				EnvoyID: envoyID,
			},
		},
		Kind: api.ServiceKindConnectProxy,
	}

	extension := builtinextensiontemplate.EnvoyExtension{Constructor: validate.MakeValidate}
	err := extension.Validate(extConfig)
	if err != nil {
		return err
	}

	_, err = extension.Extend(indexedResources, extConfig)
	if err != nil {
		return err
	}

	v, ok := extension.Plugin.(*validate.Validate)
	if !ok {
		panic("validate plugin was not correctly created")
	}

	return v.Errors(clusters)
}

func proxyConfigDumpToIndexedResources(config *envoy_admin_v3.ConfigDump) (*xdscommon.IndexedResources, error) {
	indexedResources := xdscommon.EmptyIndexedResources()

	for _, cfg := range config.Configs {
		switch cfg.TypeUrl {
		case listenersType:
			lcd := &envoy_admin_v3.ListenersConfigDump{}

			err := proto.Unmarshal(cfg.GetValue(), lcd)
			if err != nil {
				return indexedResources, err
			}

			for _, listener := range lcd.GetDynamicListeners() {
				// TODO We should care about these:
				// listener.GetErrorState()
				// listener.GetDrainingState()
				// listener.GetWarmingState()

				r := indexedResources.Index[xdscommon.ListenerType]
				if r == nil {
					r = make(map[string]proto.Message)
				}
				as := listener.GetActiveState()
				if as == nil {
					continue
				}

				l := &envoy_listener_v3.Listener{}
				proto.Unmarshal(as.Listener.GetValue(), l)
				if err != nil {
					return indexedResources, err
				}

				r[listener.Name] = l
				indexedResources.Index[xdscommon.ListenerType] = r
			}
		case clustersType:
			ccd := &envoy_admin_v3.ClustersConfigDump{}

			err := proto.Unmarshal(cfg.GetValue(), ccd)
			if err != nil {
				return indexedResources, err
			}

			// TODO we should care about ccd.GetDynamicWarmingClusters()
			for _, cluster := range ccd.GetDynamicActiveClusters() {
				r := indexedResources.Index[xdscommon.ClusterType]
				if r == nil {
					r = make(map[string]proto.Message)
				}

				c := &envoy_cluster_v3.Cluster{}
				proto.Unmarshal(cluster.GetCluster().Value, c)
				if err != nil {
					return indexedResources, err
				}

				r[c.Name] = c
				indexedResources.Index[xdscommon.ClusterType] = r
			}
		case routesType:
			rcd := &envoy_admin_v3.RoutesConfigDump{}

			err := proto.Unmarshal(cfg.GetValue(), rcd)
			if err != nil {
				return indexedResources, err
			}

			for _, route := range rcd.GetDynamicRouteConfigs() {
				r := indexedResources.Index[xdscommon.RouteType]
				if r == nil {
					r = make(map[string]proto.Message)
				}

				rc := &envoy_route_v3.RouteConfiguration{}
				proto.Unmarshal(route.GetRouteConfig().Value, rc)
				if err != nil {
					return indexedResources, err
				}

				r[rc.Name] = rc
				indexedResources.Index[xdscommon.RouteType] = r
			}
		case endpointsType:
			ecd := &envoy_admin_v3.EndpointsConfigDump{}

			err := proto.Unmarshal(cfg.GetValue(), ecd)
			if err != nil {
				return indexedResources, err
			}

			for _, route := range ecd.GetDynamicEndpointConfigs() {
				r := indexedResources.Index[xdscommon.EndpointType]
				if r == nil {
					r = make(map[string]proto.Message)
				}

				rc := &envoy_endpoint_v3.ClusterLoadAssignment{}
				err := proto.Unmarshal(route.EndpointConfig.GetValue(), rc)
				if err != nil {
					return indexedResources, err
				}

				r[rc.ClusterName] = rc
				indexedResources.Index[xdscommon.EndpointType] = r
			}
		}
	}

	return indexedResources, nil
}
