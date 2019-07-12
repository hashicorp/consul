services {
  name = "s1"
  port = 8080

  connect {
    sidecar_service {
      port = 21000

      proxy {
        upstreams = [
          {
            destination_name = "s2"
            local_bind_port  = 5000
          },
        ]
      }
    }
  }
}
