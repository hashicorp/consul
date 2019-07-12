services {
  name = "s2"

  # Advertise gRPC port
  port = 8179

  connect {
    sidecar_service {
      port = 21001

      proxy {
        config {
          protocol = "grpc"
        }
      }
    }
  }
}
