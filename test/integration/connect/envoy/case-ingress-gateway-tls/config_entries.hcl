enable_central_service_config = true

config_entries {
  bootstrap {
    kind = "ingress-gateway"
    name = "ingress-gateway"

    tls {
      enabled = true
    }

    listeners = [
      {
        port = 9999
        protocol = "tcp"
        services = [
          {
            name = "s1"
          }
        ]
      }
    ]
  }
}
