package wait

import (
	"fmt"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil/retry"
)

type rpcFn func(string, interface{}, interface{}) error

func ForLeader(t retry.Fataler, rpc rpcFn, dc string) {
	retry.Fatal(t, func() error {
		var out structs.IndexedNodes
		args := &structs.DCSpecificRequest{Datacenter: dc}
		if err := rpc("Catalog.ListNodes", args, &out); err != nil {
			return fmt.Errorf("Catalog.ListNodes failed: %v", err)
		}
		if !out.QueryMeta.KnownLeader {
			return fmt.Errorf("no leader")
		}
		if out.Index == 0 {
			return fmt.Errorf("consul index is 0")
		}
		return nil
	})
}
