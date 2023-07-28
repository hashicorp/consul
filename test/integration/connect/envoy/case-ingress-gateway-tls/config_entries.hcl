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

      tls {
        enabled = true
      }

      listeners = [
        {
          port     = 9998
          protocol = "http"
          services = [
            {
              name = "s1"
            }
          ]
        },
        {
          port     = 9999
          protocol = "http"
          services = [
            {
              name  = "s1"
              hosts = ["test.example.com"]
            }
          ]
        }
      ]
    }
  ]
}
