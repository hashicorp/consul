enable_central_service_config = true

config_entries {
  bootstrap = [
    {
      kind = "ingress-gateway"
      name = "ingress-gateway"

      listeners = [
        {
          port = 9999
          protocol = "http"
          services = [
            {
              name = "router"
            }
          ]
        }
      ]
    },
    {
      kind = "proxy-defaults"
      name = "global"
      config {
        protocol = "http"
      }
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
            service = "s1"
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
            service = "s2"
            prefix_rewrite = "/"
          }
        }
      ]
    }
  ]
}
