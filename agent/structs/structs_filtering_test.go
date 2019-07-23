package structs

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/consul/api"
	bexpr "github.com/hashicorp/go-bexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dumpFieldConfig = flag.Bool("dump-field-config", false, "generate field config dump file")

///////////////////////////////////////////////////////////////////////////////
//
// NOTE: The tests within this file are designed to validate that the fields
//       that will be available for filtering for various data types in the
//       structs package have the correct field configurations. If you need
//       to update this file to get the tests passing again then you definitely
//       should update the documentation as well.
//
///////////////////////////////////////////////////////////////////////////////

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
	"MeshGateway": &bexpr.FieldConfiguration{
		StructFieldName: "MeshGateway",
		SubFields:       expectedFieldConfigMeshGatewayConfig,
	},
}

var expectedFieldConfigConnectProxyConfig bexpr.FieldConfigurations = bexpr.FieldConfigurations{
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
	"Upstreams": &bexpr.FieldConfiguration{
		StructFieldName:     "Upstreams",
		SupportedOperations: []bexpr.MatchOperator{bexpr.MatchIsEmpty, bexpr.MatchIsNotEmpty},
		SubFields:           expectedFieldConfigUpstreams,
	},
	"MeshGateway": &bexpr.FieldConfiguration{
		StructFieldName: "MeshGateway",
		SubFields:       expectedFieldConfigMeshGatewayConfig,
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

// Only need to generate the field configurations for the top level filtered types
// The internal types will be checked within these.
var fieldConfigTests map[string]fieldConfigTest = map[string]fieldConfigTest{
	"Node": fieldConfigTest{
		dataType: (*Node)(nil),
		expected: expectedFieldConfigNode,
	},
	"NodeService": fieldConfigTest{
		dataType: (*NodeService)(nil),
		expected: expectedFieldConfigNodeService,
	},
	"ServiceNode": fieldConfigTest{
		dataType: (*ServiceNode)(nil),
		expected: expectedFieldConfigServiceNode,
	},
	"HealthCheck": fieldConfigTest{
		dataType: (*HealthCheck)(nil),
		expected: expectedFieldConfigHealthCheck,
	},
	"CheckServiceNode": fieldConfigTest{
		dataType: (*CheckServiceNode)(nil),
		expected: expectedFieldConfigCheckServiceNode,
	},
	"NodeInfo": fieldConfigTest{
		dataType: (*NodeInfo)(nil),
		expected: expectedFieldConfigNodeInfo,
	},
	"api.AgentService": fieldConfigTest{
		dataType: (*api.AgentService)(nil),
		// this also happens to ensure that our API representation of a service that can be
		// registered with an agent stays in sync with our internal NodeService structure
		expected: expectedFieldConfigNodeService,
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
	t.Parallel()

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
