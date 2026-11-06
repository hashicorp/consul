// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

// TestCatalogRegister_DistinctNodeThroughput measures leader catalog write
// throughput as the number of concurrent writers grows. Each writer registers
// its own node, as client agents' anti-entropy does, so the writes don't touch
// the same catalog entries. Raft batches concurrent applies into one log
// write, so throughput should grow with the number of writers.
//
// Regression test for https://github.com/hashicorp/consul/issues/24108: the
// shared stripe-0 node write lock serialized every catalog write on the
// leader, capping throughput at the single-writer rate.
func TestCatalogRegister_DistinctNodeThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	_, s := testServerWithConfig(t)
	testrpc.WaitForLeader(t, s.RPC, "dc1")

	const duration = 3 * time.Second
	for _, writers := range []int{1, 8, 64} {
		var writes atomic.Int64
		var wg sync.WaitGroup
		start := time.Now()
		deadline := start.Add(duration)
		for w := 0; w < writers; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				node := fmt.Sprintf("node-%d-%d", writers, w)
				for i := 0; time.Now().Before(deadline); i++ {
					args := structs.RegisterRequest{
						Datacenter: "dc1",
						Node:       node,
						Address:    "127.0.0.1",
						Service: &structs.NodeService{
							ID:      "web",
							Service: "web",
							Port:    8080,
							Tags:    []string{strconv.Itoa(i)},
						},
					}
					var out struct{}
					if err := s.RPC(context.Background(), "Catalog.Register", &args, &out); err != nil {
						t.Error(err)
						return
					}
					writes.Add(1)
				}
			}(w)
		}
		wg.Wait()
		elapsed := time.Since(start)
		n := writes.Load()
		t.Logf("writers=%-2d  writes/s=%5.0f  mean latency=%v",
			writers, float64(n)/elapsed.Seconds(), (elapsed * time.Duration(writers) / time.Duration(n)).Round(10*time.Microsecond))
	}
}
