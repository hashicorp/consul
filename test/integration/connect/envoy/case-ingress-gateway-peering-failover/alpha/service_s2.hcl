services {
  name = "s2"
  port = 8181
  checks = []
  connect {
    sidecar_service {
      checks = []
    }
  }
}
