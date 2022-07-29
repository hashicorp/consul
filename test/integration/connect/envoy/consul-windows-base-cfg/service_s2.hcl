services {
  name = "s2"
  port = 8181
  connect { sidecar_service {
    proxy {
        local_service_address = "envoy_s2-sidecar-proxy_1"
      }
  } }
}