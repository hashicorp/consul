ID {
  Type = gvk("mesh.v2beta1.Destinations")
  Name = "api"
}

Data {
  Workloads {
    Prefixes = ["api"]
  }

  Destinations = [
    {
      DestinationRef = {
        Type = gvk("catalog.v2beta1.Service")
        Name = "db"
      }

      DestinationPort = "tcp"

      IpPort = {
        Port = 1234
      }
    }
  ]
}
