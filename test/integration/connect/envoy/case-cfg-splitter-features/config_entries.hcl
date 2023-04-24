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
    name = "s2"

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
    kind = "service-splitter"
    name = "s2"

    splits = [
      {
        weight         = 50,
        service_subset = "v2"
        request_headers {
          set {
            x-split-leg = "v2"
          }
          remove = ["x-bad-req"]
        }
        response_headers {
          add {
            x-svc-version = "v2"
          }
          remove = ["x-bad-resp"]
        }
      },
      {
        weight         = 50,
        service_subset = "v1"
        request_headers {
          set {
            x-split-leg = "v1"
          }
          remove = ["x-bad-req"]
        }
        response_headers {
          add {
            x-svc-version = "v1"
          }
          remove = ["x-bad-resp"]
        }
      },
    ]
  }
}
