enable_central_service_config = true

config_entries {
  bootstrap {
    kind = "proxy-defaults"
    name = "global"

    config {
      protocol = "http"
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "s3"

    subsets = {
      "v1" = {
        filter = "Service.Meta.version == v1"
      }

      "v2" = {
        filter = "Service.Meta.version == v2"
      }
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "s2"

    failover = {
      "*" = {
        service        = "s3"
        service_subset = "v1"
      }
    }
  }
}
