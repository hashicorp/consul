services {
  name = "s2"
  # Advertise gRPC (http2) port
  port = 8179
  connect {
    sidecar_service {
      proxy {
        config {
          protocol = "http2"
        }
      }
    }
  }
}