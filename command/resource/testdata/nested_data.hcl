# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
        Ip   = "127.0.0.1"
        Port = 1234
      }
    }
  ]
}
