schema_version = 1

project {
  license		= "MPL-2.0"
  copyright_year	= 2024

    # (OPTIONAL) A list of globs that should not have copyright/license headers.
  # Supports doublestar glob patterns for more flexibility in defining which
  # files or folders should be ignored
  header_ignore = [
    # Forked and modified UI libs
    "ui/packages/consul-ui/app/utils/dom/event-target/**",
    "ui/packages/consul-ui/lib/rehype-prism/**",
    "ui/packages/consul-ui/lib/block-slots/**",

    # UI file that do not render properly with copyright headers
    "ui/packages/consul-ui/app/components/brand-loader/enterprise.hbs",
    "ui/packages/consul-ui/app/components/brand-loader/index.hbs",

    # ignore specific test data files
    "agent/uiserver/testdata/**",

    # generated files 
    "agent/structs/structs.deepcopy.go",
    "agent/proxycfg/proxycfg.deepcopy.go",
    "agent/grpc-middleware/rate_limit_mappings.gen.go",
    "agent/uiserver/dist/**",
    "agent/consul/state/catalog_schema.deepcopy.go",
    "agent/config/config.deepcopy.go",
    "agent/grpc-middleware/testutil/testservice/simple.pb.go",
    "proto-public/annotations/ratelimit/ratelimit.pb.go",
    "proto-public/pbacl/acl.pb.go",
    "proto-public/pbconnectca/ca.pb.go",
    "proto-public/pbdataplane/dataplane.pb.go",
    "proto-public/pbdns/dns.pb.go",
    "proto-public/pbserverdiscovery/serverdiscovery.pb.go",
    "proto/pbacl/acl.pb.go",
    "proto/pbautoconf/auto_config.pb.go",
    "proto/pbcommon/common.pb.go",
    "proto/pbconfig/config.pb.go",
    "proto/pbconfigentry/config_entry.pb.go",
    "proto/pbconnect/connect.pb.go",
    "proto/pboperator/operator.pb.go",
    "proto/pbpeering/peering.pb.go",
    "proto/pbpeerstream/peerstream.pb.go",
    "proto/pbservice/healthcheck.pb.go",
    "proto/pbservice/node.pb.go",
    "proto/pbservice/service.pb.go",
    "proto/pbsubscribe/subscribe.pb.go",
  ]
}
