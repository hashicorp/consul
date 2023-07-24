ID = {
  Type = {
    Group = "mesh"
    GroupVersion = "v1alpha1"
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
          GroupVersion = "v1alpha1"
          Kind = "Service"
        }

        Name = "db"
      }

      DestinationPort = "tcp"

      TCP = {
        Port = 1234
      }
    }
  ]
}
