config_entries {
  bootstrap {
    kind = "proxy-defaults"
    name = "global"
    config {
      envoy_prometheus_bind_addr = "0.0.0.0:1234"
    }
  }
  bootstrap {
    kind     = "service-defaults"
    name     = "s1"
    protocol = "http"
  }
  bootstrap {
    kind     = "service-defaults"
    name     = "s2"
    protocol = "http"
  }
}
