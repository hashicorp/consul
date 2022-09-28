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

      Defaults {
        MaxConnections = 10
        MaxPendingRequests = 20
        MaxConcurrentRequests = 30
      }
      listeners = [
        {
          port     = 9999
          protocol = "http"
          services = [
            {
              name = "*"
            }
          ]
        },
        {
          port     = 9998
          protocol = "http"
          services = [
            {
              name  = "s1"
              hosts = ["test.example.com"]
              MaxConnections = 100
              MaxPendingRequests = 200
              MaxConcurrentRequests = 300
            }
          ]
        }
      ]
    }
  ]
}
