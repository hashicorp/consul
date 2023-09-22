# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

ID {
  Type = gvk("mesh.v2beta1.Upstreams")
  Name = "api"
}

Data {
  Workloads {
    Prefixes = ["api"]
  }

  Upstreams = [
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
