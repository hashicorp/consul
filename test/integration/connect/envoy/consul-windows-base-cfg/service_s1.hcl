services {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s2"
            local_bind_port = 5000
          }
        ]
        local_service_address = "envoy_s1-sidecar-proxy_1"
      }
    }
  }
}