services {
  id   = "s3-v1"
  name = "s3"
  port = 8283

  meta {
    version = "v1"
  }

  connect {
    sidecar_service {}
  }
}
