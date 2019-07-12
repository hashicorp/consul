services {
  name = "s3"
  port = 8282

  connect {
    sidecar_service {
      port = 21002
    }
  }
}
