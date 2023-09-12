# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

ID {
  Type = gvk("mesh.v1alpha1.Upstreams")
  Name = "api"
}

Data {
  Workloads {
    Prefixes = ["api"]
  }

  Upstreams = [
    {
      DestinationRef = {
        Type = gvk("catalog.v1alpha1.Service")
        Name = "db"
      }

      DestinationPort = "tcp"

      IpPort = {
        Port = 1234
      }
    }
  ]
}
