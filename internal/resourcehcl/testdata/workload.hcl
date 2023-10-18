ID {
  Type = gvk("catalog.v2beta1.Workload")
  Name = "work1"
}

Data {
  Addresses = [
    {
      Host = "127.0.0.1"
    },
  ]
  Ports = {
    "http" = {
      port = 8080
    }
  }
}

Metadata = {
  "foo" = "bar"
}