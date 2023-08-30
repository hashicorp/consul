services {
  name = "s2"
  port = 8181
  connect {
    sidecar_service {
      proxy {
        expose {
          checks = true
        }
      }
    }
  }
  checks = [
    {
      name     = "http"
      http     = "http://127.0.0.1:8181/debug"
      method   = "GET"
      interval = "10s"
      timeout  = "1s"
    },
  ]
}
