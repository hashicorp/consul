node_name = "sec"
connect {
  enabled                            = true
  enable_mesh_gateway_wan_federation = true
}
primary_gateways = [
  "consul-primary:4431",
]
primary_gateways_interval = "5s"
ca_file                   = "/workdir/secondary/tls/consul-agent-ca.pem"
cert_file                 = "/workdir/secondary/tls/secondary-server-consul-0.pem"
key_file                  = "/workdir/secondary/tls/secondary-server-consul-0-key.pem"
verify_incoming           = true
verify_outgoing           = true
verify_server_hostname    = true
