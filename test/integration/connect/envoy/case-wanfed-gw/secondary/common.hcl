tls {
  internal_rpc {
    ca_file                = "/workdir/secondary/tls/consul-agent-ca.pem"
    cert_file              = "/workdir/secondary/tls/secondary-server-consul-0.pem"
    key_file               = "/workdir/secondary/tls/secondary-server-consul-0-key.pem"
    verify_incoming        = true
    verify_outgoing        = true
    verify_server_hostname = true
  }
}
