services {
  id   = "s3-v2"
  name = "s3"
  port = 8284

  meta {
    version = "v2"
  }

  connect {
    sidecar_service {}
  }
}
