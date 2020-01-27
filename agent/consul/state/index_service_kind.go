package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// IndexServiceKind indexes a *struct.ServiceNode for querying by
// the services kind. We need a custom indexer because of the default
// kind being the empty string. The StringFieldIndex in memdb seems to
// treate the empty string as missing and doesn't work correctly when we actually
// want to index ""
type IndexServiceKind struct{}

func (idx *IndexServiceKind) FromObject(obj interface{}) (bool, []byte, error) {
	sn, ok := obj.(*structs.ServiceNode)
	if !ok {
		return false, nil, fmt.Errorf("Object must be ServiceNode, got %T", obj)
	}

	return true, append([]byte(strings.ToLower(string(sn.ServiceKind))), '\x00'), nil
}

func (idx *IndexServiceKind) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}

	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a structs.ServiceKind: %#v", args[0])
	}

	// Add the null character as a terminator
	return append([]byte(strings.ToLower(arg)), '\x00'), nil
}
