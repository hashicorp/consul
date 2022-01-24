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
              name = "*"
            }
          ]
          tls {
            sds {
              cluster_name = "sds-cluster"
              cert_resource = "wildcard.ingress.consul"
            }
          }
        },
        {
          port     = 9998
          protocol = "http"
          services = [
            {
              name  = "s1"
              hosts = ["foo.example.com"]
                tls {
                sds {
                  cluster_name = "sds-cluster"
                  cert_resource = "foo.example.com"
                }
              }
            },
            {
              # Route to s2 on a differet domain with different cert
              name  = "s2"
              hosts = ["www.example.com"]
                tls {
                sds {
                  cluster_name = "sds-cluster"
                  cert_resource = "www.example.com"
                }
              }
            }
          ]
        }
      ]
    }
  ]
}
