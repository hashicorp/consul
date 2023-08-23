config_entries {
  bootstrap {
    kind = "proxy-defaults"
    name = "global"

    config {
      protocol = "tcp"
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "s2"

    redirect {
      service = "s3"
    }
  }
}
