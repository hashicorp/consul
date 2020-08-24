services {
  name = "s2"
  # Advertise gRPC port
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
