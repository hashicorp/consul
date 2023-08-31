services {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s2"
            datacenter = "secondary"
            local_bind_port = 5000
            mesh_gateway {
              mode = "local"
            }
          }
        ]
      }
    }
  }
}