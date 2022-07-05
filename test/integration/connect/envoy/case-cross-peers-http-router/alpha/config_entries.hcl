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
      kind = "service-router"
      name = "s2"
      routes = [
        {
          match { http { path_prefix = "/s3/" } }
          destination {
            service        = "s3"
            prefix_rewrite = "/"
          }
        },
      ]
    },
    {
      kind = "exported-services"
      name = "default"
      services = [
        {
          name = "s2"
          consumers = [
            {
              peer_name = "alpha-to-primary"
            }
          ]
        }
      ]
    }
  ]
}
