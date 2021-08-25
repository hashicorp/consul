config_entries {
  bootstrap = [
    {
      kind = "proxy-defaults"
      name = "global"
      config {
        protocol = "http"
      }
    },
    {
      kind = "ingress-gateway"
      name = "ingress-gateway"

      listeners = [
        {
          port     = 9999
          protocol = "http"
          services = [
            {
              name = "router"
              request_headers {
                add {
                  x-foo = "bar-req"
                  x-existing-1 = "appended-req"
                }
                set {
                  x-existing-2 = "replaced-req"
                  x-client-ip = "%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT%"
                }
                remove = ["x-bad-req"]
              }
              response_headers {
                add {
                  x-foo = "bar-resp"
                  x-existing-1 = "appended-resp"
                }
                set {
                  x-existing-2 = "replaced-resp"
                }
                remove = ["x-bad-resp"]
              }
            }
          ]
        }
      ]
    },
    {
      kind = "service-router"
      // This is a "virtual" service name and will not have a backing
      // service definition. It must match the name defined in the ingress
      // configuration.
      name = "router"
      routes = [
        {
          match {
            http {
              path_prefix = "/s1/"
            }
          }

          destination {
            service        = "s1"
            prefix_rewrite = "/"
          }
        },
        {
          match {
            http {
              path_prefix = "/s2/"
            }
          }

          destination {
            service        = "s2"
            prefix_rewrite = "/"
          }
        }
      ]
    }
  ]
}
