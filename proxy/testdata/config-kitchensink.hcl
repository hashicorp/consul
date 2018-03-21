# Example proxy config with everything specified

proxy_id = "foo"
token = "11111111-2222-3333-4444-555555555555"

proxied_service_name = "web"
proxied_service_namespace = "default"

# Assumes running consul in dev mode from the repo root...
dev_ca_file = "connect/testdata/ca1-ca-consul-internal.cert.pem"
dev_service_cert_file = "connect/testdata/ca1-svc-web.cert.pem"
dev_service_key_file = "connect/testdata/ca1-svc-web.key.pem"

public_listener {
  bind_address = ":9999"
  local_service_address = "127.0.0.1:5000"
  local_connect_timeout_ms = 1000
  handshake_timeout_ms = 5000
}

upstreams = [
  {
    local_bind_address = "127.0.0.1:6000"
    destination_name = "db"
    destination_namespace = "default"
    destination_type = "service"
    connect_timeout_ms = 10000
  },
  {
    local_bind_address = "127.0.0.1:6001"
    destination_name = "geo-cache"
    destination_namespace = "default"
    destination_type = "prepared_query"
    connect_timeout_ms = 10000
  }
]
