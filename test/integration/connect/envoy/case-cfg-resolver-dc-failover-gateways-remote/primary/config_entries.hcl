config_entries {
  bootstrap {
    kind = "service-defaults"
    name = "s2"

    protocol = "http"

    mesh_gateway {
      mode = "remote"
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "s2"

    failover = {
      "*" = {
        datacenters = ["secondary"]
      }
    }
  }
}
