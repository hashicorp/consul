services {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        config {
          protocol = "http"
        }
      }
    }
  }
}
