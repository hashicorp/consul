services {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s2"
            local_bind_port  = 5000
          },
          {
            destination_name = "virtual-s2"
            local_bind_port  = 5001
          }
        ]
      }
    }
  }
}
