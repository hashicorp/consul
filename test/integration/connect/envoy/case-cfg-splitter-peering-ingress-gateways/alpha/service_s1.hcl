services {
  name = "s1"
  port = 8080
  checks = []
  connect {
    sidecar_service {
      checks = []
    }
  }
}
