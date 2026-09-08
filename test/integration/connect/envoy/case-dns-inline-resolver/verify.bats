#!/usr/bin/env bats

load helpers

@test "s1 proxy is running correct version" {
  assert_envoy_version 19000
}

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s2 proxy should be healthy" {
  assert_service_has_healthy_instances s2 1
}

@test "s1 upstream should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
}

#######################################
# Inline virtual DNS listener (:8653) #
#######################################

@test "s1 has the inline virtual DNS listener bound to loopback:8653 over UDP" {
  LISTENER=$(get_envoy_dns_listener localhost:19000 virtual_dns)

  SA=$(echo "$LISTENER" | jq -r '.address.socket_address')
  [ "$(echo "$SA" | jq -r '.port_value')" = "8653" ]
  [ "$(echo "$SA" | jq -r '.protocol')" = "UDP" ]
  ADDR=$(echo "$SA" | jq -r '.address')
  [ "$ADDR" = "127.0.0.1" ] || [ "$ADDR" = "::1" ]

  # A dns_filter UDP listener must declare a udp_listener_config.
  [ "$(echo "$LISTENER" | jq -r 'has("udp_listener_config")')" = "true" ]
}

@test "s1 inline virtual DNS listener uses the dns_filter with the virtual stat prefix" {
  LISTENER=$(get_envoy_dns_listener localhost:19000 virtual_dns)

  FILTER=$(echo "$LISTENER" | jq -r '.listener_filters[0]')
  [ "$(echo "$FILTER" | jq -r '.name')" = "envoy.filters.udp.dns_filter" ]
  [ "$(echo "$FILTER" | jq -r '.typed_config.stat_prefix')" = "consul_virtual_dns" ]
}

@test "s1 inline virtual DNS table maps s2's expanded FQDN to its virtual IP" {
  LISTENER=$(get_envoy_dns_listener localhost:19000 virtual_dns)

  FQDN="s2.virtual.default.ns.default.ap.primary.dc.consul"
  DOMAIN=$(echo "$LISTENER" | jq -c \
    ".listener_filters[0].typed_config.server_config.inline_dns_table.virtual_domains[] | select(.name == \"${FQDN}\")")
  [ -n "$DOMAIN" ]

  # A VIP is advertised for the FQDN.
  ADDRS=$(echo "$DOMAIN" | jq -r '.endpoint.address_list.address[]')
  [ -n "$ADDRS" ]
}

@test "s1 inline virtual DNS table is stable across config dumps" {
  FIRST=$(get_envoy_dns_listener localhost:19000 virtual_dns \
    | jq -S '.listener_filters[0].typed_config.server_config.inline_dns_table')
  SECOND=$(get_envoy_dns_listener localhost:19000 virtual_dns \
    | jq -S '.listener_filters[0].typed_config.server_config.inline_dns_table')
  [ "$FIRST" = "$SECOND" ]
}

######################################
# Egress recursor DNS listener (:8654)
######################################

@test "s1 has the egress recursor DNS listener bound to loopback:8654 over UDP" {
  LISTENER=$(get_envoy_dns_listener localhost:19000 egress_dns)

  SA=$(echo "$LISTENER" | jq -r '.address.socket_address')
  [ "$(echo "$SA" | jq -r '.port_value')" = "8654" ]
  [ "$(echo "$SA" | jq -r '.protocol')" = "UDP" ]
  ADDR=$(echo "$SA" | jq -r '.address')
  [ "$ADDR" = "127.0.0.1" ] || [ "$ADDR" = "::1" ]

  [ "$(echo "$LISTENER" | jq -r 'has("udp_listener_config")')" = "true" ]
}

@test "s1 egress recursor DNS listener forwards to the configured recursors via c-ares" {
  LISTENER=$(get_envoy_dns_listener localhost:19000 egress_dns)

  CFG=$(echo "$LISTENER" | jq -r '.listener_filters[0].typed_config')
  [ "$(echo "$CFG" | jq -r '.stat_prefix')" = "consul_egress_dns" ]

  # Forwarding-only: an (empty) inline table plus a c-ares client config.
  [ "$(echo "$CFG" | jq -r '.client_config.typed_dns_resolver_config.name')" = "envoy.network.dns_resolver.cares" ]

  RESOLVERS=$(echo "$CFG" | jq -r \
    '.client_config.typed_dns_resolver_config.typed_config.resolvers[].socket_address.address' | sort | tr '\n' ' ')
  [ "$RESOLVERS" = "8.8.4.4 8.8.8.8 " ]
}
