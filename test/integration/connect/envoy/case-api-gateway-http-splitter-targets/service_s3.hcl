services {
  id   = "s3"
  name = "s3"
  port = 8182

  connect {
    sidecar_service {}
  }
}
