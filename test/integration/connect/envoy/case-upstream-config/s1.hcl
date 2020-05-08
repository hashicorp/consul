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
            config {
              limits {
                max_connections = 3
                max_pending_requests = 4
                max_concurrent_requests = 5
              }
              passive_health_check {
                interval = "22s"
                max_failures = 4
              }
            }
          }
        ]
      }
    }
  }
}
