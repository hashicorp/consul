ID = {
  Type = {
    Group = "mesh"
    GroupVersion = "v2beta1"
    Kind = "Upstreams"
  }
}

Data = {
  Workloads = {
    Prefixes = ["api"]
  }

  Upstreams = [
    {
      DestinationRef = {
        Type = {
          Group = "catalog"
          GroupVersion = "v2beta1"
          Kind = "Service"
        }

        Name = "db"
      }

      DestinationPort = "tcp"

      IpPort = {
        Port = 1234
      }
    }
  ]
}
