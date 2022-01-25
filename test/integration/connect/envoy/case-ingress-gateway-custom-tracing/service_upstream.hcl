services {
  name = "upstream"
  port = 8079
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
