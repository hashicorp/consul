config_entries {
  bootstrap {
    kind = "proxy-defaults"
    name = "global"

    config {
      protocol = "tcp"
    }
  }

  bootstrap {
    kind = "ingress-gateway"
    name = "ingress-gateway"
    listeners = [
      {
        protocol = "tcp"
        port     = 10000
        services = [
          {
            name = "s2"
          }
        ]
      }
    ]
  }

  bootstrap {
    kind = "service-resolver"
    name = "s2"

    failover = {
      "*" = {
        targets = [{peer = "primary-to-alpha"}]
      }
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "virtual-s2"

    redirect = {
      service = "s2"
      peer = "primary-to-alpha"
    }
  }
}
