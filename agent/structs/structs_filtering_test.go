// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/go-bexpr"
	"github.com/mitchellh/pointerstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

var dumpFieldConfig = flag.Bool("dump-field-config", false, "generate field config dump file")

func TestPointerStructure(t *testing.T) {
	csn := CheckServiceNode{
		Node: &Node{
			ID:      "f18f3a10-2153-40ae-af7d-68db0e856498",
			Node:    "node1",
			Address: "198.18.0.1",
		},
		Service: &NodeService{
			ID:      "test",
			Service: "test",
			Port:    1234,
			TaggedAddresses: map[string]ServiceAddress{
				"wan": {
					Address: "1.1.1.1",
					Port:    443,
				},
			},
		},
	}

	ptr := pointerstructure.Pointer{
		Parts: []string{
			"Service",
			"TaggedAddresses",
			"wan",
			"Address",
		},
	}

	val, err := ptr.Get(csn)
	require.NoError(t, err)
	require.Equal(t, "1.1.1.1", val)
}

// /////////////////////////////////////////////////////////////////////////////
//
// NOTE: The tests within this file are designed to validate that the fields
//       that will be available for filtering for various data types in the
//       structs package have the correct field configurations. If you need
//       to update this file to get the tests passing again then you definitely
//       should update the documentation as well.
//
// /////////////////////////////////////////////////////////////////////////////

type fieldConfigTest struct {
	dataType interface{}
	expected bexpr.FieldConfigurations
}

// ----------------------------------------------------------------------------
//
// The following are not explicitly tested as they are supporting structures
// nested within the other API responses
//
// ----------------------------------------------------------------------------

var expectedFieldConfigServiceAddress bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Address": &bexpr.FieldConfiguration{
		StructFieldName:     "Address",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Port": &bexpr.FieldConfiguration{
		StructFieldName:     "Port",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
}

var expectedFieldConfigMeshGatewayConfig bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Mode": &bexpr.FieldConfiguration{
		StructFieldName:     "Mode",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
}

var expectedFieldConfigTransparentProxyConfig bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"OutboundListenerPort": &bexpr.FieldConfiguration{
		StructFieldName:     "OutboundListenerPort",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"DialedDirectly": &bexpr.FieldConfiguration{
		StructFieldName:     "DialedDirectly",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
}

var expectedFieldConfigAccessLogsConfig bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Enabled": &bexpr.FieldConfiguration{
		StructFieldName:     "Enabled",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"DisableListenerLogs": &bexpr.FieldConfiguration{
		StructFieldName:     "DisableListenerLogs",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"Type": &bexpr.FieldConfiguration{
		StructFieldName:     "Type",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Path": &bexpr.FieldConfiguration{
		StructFieldName:     "Path",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"JSONPath": &bexpr.FieldConfiguration{
		StructFieldName:     "JSONPath",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"TextPath": &bexpr.FieldConfiguration{
		StructFieldName:     "TextPath",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
}

var expectedFieldConfigExposeConfig bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Checks": &bexpr.FieldConfiguration{
		StructFieldName:     "Checks",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"Paths": &bexpr.FieldConfiguration{
		StructFieldName:     "Paths",
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty},
		SubFields:           expectedFieldConfigPaths,
	},
}

var expectedFieldConfigPaths bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"ListenerPort": &bexpr.FieldConfiguration{
		StructFieldName:     "ListenerPort",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"Path": &bexpr.FieldConfiguration{
		StructFieldName:     "Path",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"LocalPathPort": &bexpr.FieldConfiguration{
		StructFieldName:     "LocalPathPort",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"Protocol": &bexpr.FieldConfiguration{
		StructFieldName:     "Protocol",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"ParsedFromCheck": &bexpr.FieldConfiguration{
		StructFieldName:     "ParsedFromCheck",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
}

var expectedFieldConfigEnvoyExtensions bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Name": &bexpr.FieldConfiguration{
		StructFieldName:     "Name",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Required": &bexpr.FieldConfiguration{
		StructFieldName:     "Required",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"ConsulVersion": &bexpr.FieldConfiguration{
		StructFieldName:     "ConsulVersion",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"EnvoyVersion": &bexpr.FieldConfiguration{
		StructFieldName:     "EnvoyVersion",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
}
var expectedFieldConfigUpstreams bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"DestinationType": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationType",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"DestinationNamespace": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationNamespace",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"DestinationPartition": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationPartition",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"DestinationPeer": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationPeer",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"DestinationName": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Datacenter": &bexpr.FieldConfiguration{
		StructFieldName:     "Datacenter",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"LocalBindAddress": &bexpr.FieldConfiguration{
		StructFieldName:     "LocalBindAddress",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"LocalBindPort": &bexpr.FieldConfiguration{
		StructFieldName:     "LocalBindPort",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"LocalBindSocketPath": &bexpr.FieldConfiguration{
		StructFieldName:     "LocalBindSocketPath",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"LocalBindSocketMode": &bexpr.FieldConfiguration{
		StructFieldName:     "LocalBindSocketMode",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"MeshGateway": &bexpr.FieldConfiguration{
		StructFieldName: "MeshGateway",
		SubFields:       expectedFieldConfigMeshGatewayConfig,
	},
}

var expectedFieldConfigConnectProxyConfig bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"EnvoyExtensions": &bexpr.FieldConfiguration{
		StructFieldName:     "EnvoyExtensions",
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty},
		SubFields:           expectedFieldConfigEnvoyExtensions,
	},
	"DestinationServiceName": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationServiceName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"DestinationServiceID": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationServiceID",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"LocalServiceAddress": &bexpr.FieldConfiguration{
		StructFieldName:     "LocalServiceAddress",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"LocalServicePort": &bexpr.FieldConfiguration{
		StructFieldName:     "LocalServicePort",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"LocalServiceSocketPath": &bexpr.FieldConfiguration{
		StructFieldName:     "LocalServiceSocketPath",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Upstreams": &bexpr.FieldConfiguration{
		StructFieldName:     "Upstreams",
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty},
		SubFields:           expectedFieldConfigUpstreams,
	},
	"MeshGateway": &bexpr.FieldConfiguration{
		StructFieldName: "MeshGateway",
		SubFields:       expectedFieldConfigMeshGatewayConfig,
	},
	"Expose": &bexpr.FieldConfiguration{
		StructFieldName: "Expose",
		SubFields:       expectedFieldConfigExposeConfig,
	},
	"TransparentProxy": &bexpr.FieldConfiguration{
		StructFieldName: "TransparentProxy",
		SubFields:       expectedFieldConfigTransparentProxyConfig,
	},
	"Mode": &bexpr.FieldConfiguration{
		StructFieldName:     "Mode",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"AccessLogs": &bexpr.FieldConfiguration{
		StructFieldName: "AccessLogs",
		SubFields:       expectedFieldConfigAccessLogsConfig,
	},
}

var expectedFieldConfigServiceConnect bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Native": &bexpr.FieldConfiguration{
		StructFieldName:     "Native",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
}

var expectedFieldConfigWeights bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Passing": &bexpr.FieldConfiguration{
		StructFieldName:     "Passing",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"Warning": &bexpr.FieldConfiguration{
		StructFieldName:     "Warning",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
}

var expectedFieldConfigMapStringValue bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	bexpr.FieldNameAny: &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
}

var expectedFieldConfigMapStringServiceAddress bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	bexpr.FieldNameAny: &bexpr.FieldConfiguration{
		SubFields: expectedFieldConfigServiceAddress,
	},
}

// ----------------------------------------------------------------------------
//
// The following structures are within the test table as they are structures
// that will be sent back at the top level of API responses
//
// ----------------------------------------------------------------------------

var expectedFieldConfigNode bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"ID": &bexpr.FieldConfiguration{
		StructFieldName:     "ID",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Node": &bexpr.FieldConfiguration{
		StructFieldName:     "Node",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Partition": &bexpr.FieldConfiguration{
		StructFieldName:     "Partition",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"PeerName": &bexpr.FieldConfiguration{
		StructFieldName:     "PeerName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Address": &bexpr.FieldConfiguration{
		StructFieldName:     "Address",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Datacenter": &bexpr.FieldConfiguration{
		StructFieldName:     "Datacenter",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"TaggedAddresses": &bexpr.FieldConfiguration{
		StructFieldName:     "TaggedAddresses",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields: bexpr.FieldConfigurations{
			bexpr.FieldNameAny: &bexpr.FieldConfiguration{
				CoerceFn:            bexpr.CoerceString,
				SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
			},
		},
	},
	"Meta": &bexpr.FieldConfiguration{
		StructFieldName:     "Meta",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields: bexpr.FieldConfigurations{
			bexpr.FieldNameAny: &bexpr.FieldConfiguration{
				CoerceFn:            bexpr.CoerceString,
				SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
			},
		},
	},
}

var expectedFieldConfigNodeService bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Kind": &bexpr.FieldConfiguration{
		StructFieldName:     "Kind",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"ID": &bexpr.FieldConfiguration{
		StructFieldName:     "ID",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Service": &bexpr.FieldConfiguration{
		StructFieldName:     "Service",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Tags": &bexpr.FieldConfiguration{
		StructFieldName:     "Tags",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
	},
	"Address": &bexpr.FieldConfiguration{
		StructFieldName:     "Address",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"TaggedAddresses": &bexpr.FieldConfiguration{
		StructFieldName:     "TaggedAddresses",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringServiceAddress,
	},
	"Meta": &bexpr.FieldConfiguration{
		StructFieldName:     "Meta",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringValue,
	},
	"Port": &bexpr.FieldConfiguration{
		StructFieldName:     "Port",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"SocketPath": &bexpr.FieldConfiguration{
		StructFieldName:     "SocketPath",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Weights": &bexpr.FieldConfiguration{
		StructFieldName: "Weights",
		SubFields:       expectedFieldConfigWeights,
	},
	"EnableTagOverride": &bexpr.FieldConfiguration{
		StructFieldName:     "EnableTagOverride",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"Proxy": &bexpr.FieldConfiguration{
		StructFieldName: "Proxy",
		SubFields:       expectedFieldConfigConnectProxyConfig,
	},
	"ServiceConnect": &bexpr.FieldConfiguration{
		StructFieldName: "ServiceConnect",
		SubFields:       expectedFieldConfigServiceConnect,
	},
	"PeerName": &bexpr.FieldConfiguration{
		StructFieldName:     "PeerName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
}

var expectedFieldConfigServiceNode bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"ID": &bexpr.FieldConfiguration{
		StructFieldName:     "ID",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Node": &bexpr.FieldConfiguration{
		StructFieldName:     "Node",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Address": &bexpr.FieldConfiguration{
		StructFieldName:     "Address",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Datacenter": &bexpr.FieldConfiguration{
		StructFieldName:     "Datacenter",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"TaggedAddresses": &bexpr.FieldConfiguration{
		StructFieldName:     "TaggedAddresses",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringValue,
	},
	"NodeMeta": &bexpr.FieldConfiguration{
		StructFieldName:     "NodeMeta",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringValue,
	},
	"ServiceKind": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceKind",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"ServiceID": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceID",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"ServiceName": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"ServiceTags": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceTags",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
	},
	"ServiceAddress": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceAddress",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"ServiceTaggedAddresses": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceTaggedAddresses",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringServiceAddress,
	},
	"ServiceMeta": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceMeta",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringValue,
	},
	"ServicePort": &bexpr.FieldConfiguration{
		StructFieldName:     "ServicePort",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"ServiceSocketPath": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceSocketPath",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"ServiceWeights": &bexpr.FieldConfiguration{
		StructFieldName: "ServiceWeights",
		SubFields:       expectedFieldConfigWeights,
	},
	"ServiceEnableTagOverride": &bexpr.FieldConfiguration{
		StructFieldName:     "ServiceEnableTagOverride",
		CoerceFn:            bexpr.CoerceBool,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"ServiceProxy": &bexpr.FieldConfiguration{
		StructFieldName: "ServiceProxy",
		SubFields:       expectedFieldConfigConnectProxyConfig,
	},
	"ServiceConnect": &bexpr.FieldConfiguration{
		StructFieldName: "ServiceConnect",
		SubFields:       expectedFieldConfigServiceConnect,
	},
	"PeerName": &bexpr.FieldConfiguration{
		StructFieldName:     "PeerName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
}

var expectedFieldConfigHealthCheck bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Node": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "Node",
	},
	"CheckId": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "CheckId",
	},
	"Name": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "Name",
	},
	"Status": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "Status",
	},
	"Notes": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "Notes",
	},
	"Output": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "Output",
	},
	"ServiceID": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "ServiceID",
	},
	"ServiceName": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "ServiceName",
	},
	"ServiceTags": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		StructFieldName:     "ServiceTags",
	},
	"Type": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "Type",
	},

	"Interval": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "Interval",
	},

	"Timeout": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "Timeout",
	},

	"ExposedPort": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
		StructFieldName:     "ExposedPort",
	},
	"PeerName": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
		StructFieldName:     "PeerName",
	},
	"LastCheckStartTime": &bexpr.FieldConfiguration{
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{},
		StructFieldName:     "LastCheckStartTime",
	},
}

var expectedFieldConfigCheckServiceNode bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"Node": &bexpr.FieldConfiguration{
		StructFieldName: "Node",
		SubFields:       expectedFieldConfigNode,
	},
	"Service": &bexpr.FieldConfiguration{
		StructFieldName: "Service",
		SubFields:       expectedFieldConfigNodeService,
	},
	"Checks": &bexpr.FieldConfiguration{
		StructFieldName:     "Checks",
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty},
		SubFields:           expectedFieldConfigHealthCheck,
	},
}

var expectedFieldConfigNodeInfo bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"ID": &bexpr.FieldConfiguration{
		StructFieldName:     "ID",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Node": &bexpr.FieldConfiguration{
		StructFieldName:     "Node",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Partition": &bexpr.FieldConfiguration{
		StructFieldName:     "Partition",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"PeerName": &bexpr.FieldConfiguration{
		StructFieldName:     "PeerName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Address": &bexpr.FieldConfiguration{
		StructFieldName:     "Address",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"TaggedAddresses": &bexpr.FieldConfiguration{
		StructFieldName:     "TaggedAddresses",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringValue,
	},
	"Meta": &bexpr.FieldConfiguration{
		StructFieldName:     "Meta",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringValue,
	},
	"Services": &bexpr.FieldConfiguration{
		StructFieldName:     "Services",
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty},
		SubFields:           expectedFieldConfigNodeService,
	},
	"Checks": &bexpr.FieldConfiguration{
		StructFieldName:     "Checks",
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty},
		SubFields:           expectedFieldConfigHealthCheck,
	},
}

var expectedFieldConfigIntention bexpr.FieldConfigurations = bexpr.FieldConfigurations{
	"ID": &bexpr.FieldConfiguration{
		StructFieldName:     "ID",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Description": &bexpr.FieldConfiguration{
		StructFieldName:     "Description",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"SourcePeer": &bexpr.FieldConfiguration{
		StructFieldName:     "SourcePeer",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"SourceSamenessGroup": &bexpr.FieldConfiguration{
		StructFieldName:     "SourceSamenessGroup",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"SourcePartition": &bexpr.FieldConfiguration{
		StructFieldName:     "SourcePartition",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"SourceNS": &bexpr.FieldConfiguration{
		StructFieldName:     "SourceNS",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"SourceName": &bexpr.FieldConfiguration{
		StructFieldName:     "SourceName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"DestinationPartition": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationPartition",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"DestinationNS": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationNS",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"DestinationName": &bexpr.FieldConfiguration{
		StructFieldName:     "DestinationName",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"SourceType": &bexpr.FieldConfiguration{
		StructFieldName:     "SourceType",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Action": &bexpr.FieldConfiguration{
		StructFieldName:     "Action",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual, bexpr.MatchIn, bexpr.MatchNotIn, bexpr.MatchMatches, bexpr.MatchNotMatches},
	},
	"Precedence": &bexpr.FieldConfiguration{
		StructFieldName:     "Precedence",
		CoerceFn:            bexpr.CoerceInt,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchEqual, bexpr.MatchNotEqual},
	},
	"Meta": &bexpr.FieldConfiguration{
		StructFieldName:     "Meta",
		CoerceFn:            bexpr.CoerceString,
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty, bexpr.MatchIn, bexpr.MatchNotIn},
		SubFields:           expectedFieldConfigMapStringValue,
	},
}

// Only need to generate the field configurations for the top level filtered types
// The internal types will be checked within these.
var fieldConfigTests map[string]fieldConfigTest = map[string]fieldConfigTest{
	"Node": {
		dataType: (*Node)(nil),
		expected: expectedFieldConfigNode,
	},
	"NodeService": {
		dataType: (*NodeService)(nil),
		expected: expectedFieldConfigNodeService,
	},
	"ServiceNode": {
		dataType: (*ServiceNode)(nil),
		expected: expectedFieldConfigServiceNode,
	},
	"HealthCheck": {
		dataType: (*HealthCheck)(nil),
		expected: expectedFieldConfigHealthCheck,
	},
	"CheckServiceNode": {
		dataType: (*CheckServiceNode)(nil),
		expected: expectedFieldConfigCheckServiceNode,
	},
	"NodeInfo": {
		dataType: (*NodeInfo)(nil),
		expected: expectedFieldConfigNodeInfo,
	},
	"api.AgentService": {
		dataType: (*api.AgentService)(nil),
		// this also happens to ensure that our API representation of a service that can be
		// registered with an agent stays in sync with our internal NodeService structure
		expected: expectedFieldConfigNodeService,
	},
	"Intention": {
		dataType: (*Intention)(nil),
		expected: expectedFieldConfigIntention,
	},
}

func validateFieldConfigurationsRecurse(t *testing.T, expected, actual bexpr.FieldConfigurations, path string) bool {
	t.Helper()

	ok := assert.Len(t, actual, len(expected), "Actual FieldConfigurations length of %d != expected length of %d for path %q", len(actual), len(expected), path)

	for fieldName, expectedConfig := range expected {
		actualConfig, ok := actual[fieldName]
		ok = ok && assert.True(t, ok, "Actual configuration is missing field %q", fieldName)
		ok = ok && assert.Equal(t, expectedConfig.StructFieldName, actualConfig.StructFieldName, "Field %q on path %q have different StructFieldNames - Expected: %q, Actual: %q", fieldName, path, expectedConfig.StructFieldName, actualConfig.StructFieldName)
		ok = ok && assert.ElementsMatch(t, expectedConfig.SupportedOperations, actualConfig.SupportedOperations, "Fields %q on path %q have different SupportedOperations - Expected: %v, Actual: %v", fieldName, path, expectedConfig.SupportedOperations, actualConfig.SupportedOperations)

		newPath := string(fieldName)
		if newPath == "" {
			newPath = "*"
		}
		if path != "" {
			newPath = fmt.Sprintf("%s.%s", path, newPath)
		}
		ok = ok && validateFieldConfigurationsRecurse(t, expectedConfig.SubFields, actualConfig.SubFields, newPath)

		if !ok {
			break
		}
	}

	return ok
}

func validateFieldConfigurations(t *testing.T, expected, actual bexpr.FieldConfigurations) {
	t.Helper()
	require.True(t, validateFieldConfigurationsRecurse(t, expected, actual, ""))
}

type fieldDumper struct {
	fp   *os.File
	lock sync.Mutex
}

func newFieldDumper(t *testing.T, path string) *fieldDumper {
	fp, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0660)
	require.NoError(t, err)

	return &fieldDumper{fp: fp}
}

func (d *fieldDumper) Close() {
	d.fp.Close()
}

func (d *fieldDumper) DumpFields(name string, fields bexpr.FieldConfigurations) {
	if d == nil {
		return
	}

	selectorOps := make([][2]string, 0, 10)
	// need at least 8 chars wide for "Selector"
	maxSelectorLen := 8
	// need at least 20 chars wid for "Supported Operaitons"
	maxOpsLen := 20

	fields.Walk(func(path bexpr.FieldPath, conf *bexpr.FieldConfiguration) bool {
		if len(conf.SupportedOperations) < 1 {
			return true
		}

		selector := path.String()
		var ops []string
		for _, op := range conf.SupportedOperations {
			ops = append(ops, op.String())
		}

		opString := strings.Join(ops, ", ")
		selLen := len(selector)
		opsLen := len(opString)

		if selLen > maxSelectorLen {
			maxSelectorLen = selLen
		}
		if opsLen > maxOpsLen {
			maxOpsLen = opsLen
		}

		selectorOps = append(selectorOps, [2]string{selector, opString})
		return true
	})

	sort.Slice(selectorOps, func(i, j int) bool {
		return selectorOps[i][0] < selectorOps[j][0]
	})

	d.lock.Lock()
	defer d.lock.Unlock()

	// this will print the header and the string form of the fields
	fmt.Fprintf(d.fp, "===== %s =====\n%s\n\n", name, fields)

	fmt.Fprintf(d.fp, "| %-[1]*[2]s | %-[3]*[4]s |\n", maxSelectorLen, "Selector", maxOpsLen, "Supported Operations")
	fmt.Fprintf(d.fp, "| %s | %s |\n", strings.Repeat("-", maxSelectorLen), strings.Repeat("-", maxOpsLen))
	for _, selOp := range selectorOps {
		fmt.Fprintf(d.fp, "| %-[1]*[2]s | %-[3]*[4]s |\n", maxSelectorLen, selOp[0], maxOpsLen, selOp[1])
	}
	fmt.Fprintf(d.fp, "\n")
}

func TestStructs_FilterFieldConfigurations(t *testing.T) {

	var d *fieldDumper
	if *dumpFieldConfig {
		d = newFieldDumper(t, "filter_fields.txt")
		defer d.Close()
	}

	for name, tcase := range fieldConfigTests {
		// capture these values in the closure
		name := name
		tcase := tcase
		t.Run(name, func(t *testing.T) {
			fields, err := bexpr.GenerateFieldConfigurations(tcase.dataType)
			d.DumpFields(name, fields)
			require.NoError(t, err)
			validateFieldConfigurations(t, tcase.expected, fields)
		})
	}
}

func BenchmarkStructs_FilterFieldConfigurations(b *testing.B) {
	for name, tcase := range fieldConfigTests {
		b.Run(name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				_, err := bexpr.GenerateFieldConfigurations(tcase.dataType)
				require.NoError(b, err)
			}
		})
	}
}
