//go:build !consulent
// +build !consulent

package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// SamnessGroupDefaultIndex is a placeholder for OSS. Sameness-groups are enterprise only.
type SamenessGroupDefaultIndex struct{}

var _ memdb.Indexer = (*SamenessGroupDefaultIndex)(nil)
var _ memdb.MultiIndexer = (*SamenessGroupDefaultIndex)(nil)

func (*SamenessGroupDefaultIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	return false, nil, nil
}

func (*SamenessGroupDefaultIndex) FromArgs(args ...interface{}) ([]byte, error) {
	return nil, nil
}

func checkSamenessGroup(tx ReadTxn, newConfig structs.ConfigEntry) error {
	return fmt.Errorf("sameness-groups are an enterprise-only feature")
}
