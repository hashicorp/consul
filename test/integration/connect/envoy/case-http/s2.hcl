services {
  name = "s2"
  port = 8181

  connect {
    sidecar_service {
      port = 21001

      proxy {
        config {
          protocol = "http"
        }
      }
    }
  }
}
