services {
  name = "s1"
  port = 8079
  connect {
    sidecar_service {
      proxy {
        config {
          protocol = "grpc"
        }
      }
    }
  }
}
