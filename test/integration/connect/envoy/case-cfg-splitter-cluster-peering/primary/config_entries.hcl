config_entries {
  bootstrap {
    kind = "proxy-defaults"
    name = "global"

    config {
      protocol = "http"
    }
  }

  bootstrap {
    kind = "service-splitter"
    name = "split-s2"
    splits = [
      {
        Weight = 50
        Service = "local-s2"
        ResponseHeaders {
          Set {
            "x-test-split" = "primary"
          }
        }
      },
      {
        Weight = 50
        Service = "peer-s2"
        ResponseHeaders {
          Set {
            "x-test-split" = "alpha"
          }
        }
      },
    ]
  }

  bootstrap {
    kind = "service-resolver"
    name = "local-s2"
    redirect = {
      service = "s2"
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
}
