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
              load_balancer {
                policy = "ring_hash"
                ring_hash {
                  minimum_ring_size = 3
                  maximum_ring_size = 7
                }
                hash_policy {
                  field = "header"
                  match_value = "x-my-header"
                  terminal = "true"
                }
#                hash_policy {
#                  field = "header"
#                  match_value = "x-other-header"
#                  terminal = "true"
#                }
              }
            }
          }
        ]
      }
    }
  }
}
