tls {
  defaults {
    ca_file = "/consul/config/consul-agent-ca.pem"
    cert_file = "/consul/config/consul-agent-0.pem"
    key_file = "/consul/config/consul-agent-0-key.pem"
    verify_incoming = true
    verify_outgoing = true
  }
  internal_rpc {
    verify_server_hostname = true
  }
}
