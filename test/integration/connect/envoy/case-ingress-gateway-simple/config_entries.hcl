config_entries {
  bootstrap {
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
        protocol = "tcp"
        services = [
          {
            name = "s1"
            MaxConnections = 100
            MaxPendingRequests = 200
          }
        ]
      }
    ]
  }
}
