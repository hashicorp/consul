config_entries {
  bootstrap {
    kind     = "service-defaults"
    name     = "s1"
    protocol = "grpc"
  }
  bootstrap {
    kind = "ingress-gateway"
    name = "ingress-gateway"

    listeners = [
      {
        port     = 9999
        protocol = "grpc"
        services = [
          {
            name  = "s1"
            hosts = ["localhost:9999"]
          }
        ]
      }
    ]
  }
}
