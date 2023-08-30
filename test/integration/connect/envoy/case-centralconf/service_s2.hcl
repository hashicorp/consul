services {
  name = "s2"
  port = 8181
  connect {
    sidecar_service {
      proxy {
        config {
          # We need to override this because both proxies run in same network
          # namespace and so it's non-deterministic which one manages to bind
          # the 1234 port first. This forces the issue here while still testing
          # that s1's proxy is configured from global config.
          envoy_prometheus_bind_addr = "0.0.0.0:2345"
        }
      }
    }
  }
}