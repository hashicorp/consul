services {
  name = "s1"
  port = 8079
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s2"
            local_bind_port = 5000
            config {
              protocol = "grpc"
            }
          }
        ]
        config {
          protocol = "grpc"
          envoy_dogstatsd_url = "udp://127.0.0.1:8125"
          envoy_stats_tags = ["foo=bar"]
          envoy_stats_flush_interval = "1s"
        }
      }
    }
  }
}