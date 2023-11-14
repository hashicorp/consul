ID = {
  Type = {
    Group = "mesh"
    GroupVersion = "v2beta1"
    Kind = "Destinations"
  }
}

Data = {
  Workloads = {
    Prefixes = ["api"]
  }

  Destinations = [
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
