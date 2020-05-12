config_entries {
  bootstrap {
    kind = "terminating-gateway"
    name = "terminating-gateway"

    services = [
      {
        name = "s2"
      }
    ]
  }

  bootstrap {
    kind = "proxy-defaults"
    name = "global"

    config {
      protocol = "http"
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "s2"

    default_subset = "v1"

    subsets = {
      "v1" = {
        filter = "Service.Meta.version == v1"
      }
      "v2" = {
        filter = "Service.Meta.version == v2"
      }
    }
  }
}
