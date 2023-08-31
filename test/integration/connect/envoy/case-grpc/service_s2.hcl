services {
  name = "s2"
  # Advertise gRPC port
  port = 8179
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