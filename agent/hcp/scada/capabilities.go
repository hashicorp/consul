package scada

import "github.com/hashicorp/hcp-scada-provider/capability"

// CAPCoreAPI is the capability used to securely expose the Consul HTTP API to HCP
var CAPCoreAPI = capability.NewAddr("core_api")
