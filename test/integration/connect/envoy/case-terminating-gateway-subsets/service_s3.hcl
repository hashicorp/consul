services {
  id = "s3"
  name = "s3"
  port = 8184
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s2"
            local_bind_port = 8185
          }
        ]
      }
    }
  }
}
