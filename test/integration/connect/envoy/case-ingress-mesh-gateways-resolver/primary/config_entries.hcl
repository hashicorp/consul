config_entries {
  bootstrap {
    kind = "ingress-gateway"
    name = "ingress-gateway"

    listeners = [
      {
        protocol = "tcp"
        port     = 9999
        services = [
          {
            name = "s2"
          }
        ]
      },
      {
        protocol = "tcp"
        port     = 10000
        services = [
          {
            name = "s1"
          }
        ]
      }
    ]
  }

  bootstrap {
    kind = "proxy-defaults"
    name = "global"
    mesh_gateway {
      mode = "local"
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "s2"
    redirect {
      service    = "s2"
      datacenter = "secondary"
    }
  }

  bootstrap {
    kind = "service-defaults"
    name = "s1"
    mesh_gateway {
      mode = "remote"
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "s1"
    redirect {
      service    = "s1"
      datacenter = "secondary"
    }
  }
}
