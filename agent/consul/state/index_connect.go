package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// IndexConnectService indexes a *struct.ServiceNode for querying by
// services that support Connect to some target service. This will
// properly index the proxy destination for proxies and the service name
// for native services.
type IndexConnectService struct{}

func (idx *IndexConnectService) FromObject(obj interface{}) (bool, []byte, error) {
	sn, ok := obj.(*structs.ServiceNode)
	if !ok {
		return false, nil, fmt.Errorf("Object must be ServiceNode, got %T", obj)
	}

	var result []byte
	switch {
	case sn.ServiceKind == structs.ServiceKindConnectProxy:
		// For proxies, this service supports Connect for the destination
		result = []byte(strings.ToLower(sn.ServiceProxyDestination))

	case sn.ServiceConnect.Native:
		// For native, this service supports Connect directly
		result = []byte(strings.ToLower(sn.ServiceName))

	default:
		// Doesn't support Connect at all
		return false, nil, nil
	}

	// Return the result with the null terminator appended so we can
	// differentiate prefix vs. non-prefix matches.
	return true, append(result, '\x00'), nil
}

func (idx *IndexConnectService) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}

	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}

	// Add the null character as a terminator
	return append([]byte(strings.ToLower(arg)), '\x00'), nil
}
