enable_central_service_config = true

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
          port = 9999
          protocol = "http"
          services = [
            {
              name = "*"
            }
          ]
        },
        {
          port = 9998
          protocol = "http"
          services = [
            {
              name = "s1"
              hosts = ["test.example.com"]
            }
          ]
        }
      ]
    }
  ]
}
