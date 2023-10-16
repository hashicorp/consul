config_entries {

  bootstrap {
    kind = "proxy-defaults"
    name = "global"
    config {
      protocol = "http"
    }
  }

  bootstrap {
    kind = "ingress-gateway"
    name = "ingress-gateway"
    listeners = [
      {
        protocol = "http"
        port     = 9999
        services = [
          {
            name = "peer-s2"
          }
        ]
      },
      {
        protocol = "http"
        port     = 10000
        services = [
          {
            name = "peer-s1"
          }
        ]
      },
      {
        protocol = "http"
        port     = 10001
        services = [
          {
            name = "s1"
          }
        ]
      },
      {
        protocol = "http"
        port     = 10002
        services = [
          {
            name = "split"
          }
        ]
      }
    ]
  }

  bootstrap {
    kind = "service-resolver"
    name = "peer-s1"

    redirect = {
      service = "s1"
      peer = "primary-to-alpha"
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "peer-s2"

    redirect = {
      service = "s2"
      peer = "primary-to-alpha"
    }
  }

  bootstrap {
    kind = "service-splitter"
    name = "split"
    splits = [
      {
        Weight = 50
        Service = "peer-s1"
      },
      {
        Weight = 50
        Service = "peer-s2"
      },
    ]
  }
}
