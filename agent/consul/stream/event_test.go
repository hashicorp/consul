package stream

import (
	"bytes"
	fmt "fmt"
	"testing"

	"github.com/golang/snappy"
	"github.com/hashicorp/consul/types"
)

func dummyService(i int) *Event_ServiceHealth {
	return &Event_ServiceHealth{
		ServiceHealth: &ServiceHealthUpdate{
			Op: CatalogOp_Register,
			CheckServiceNode: &CheckServiceNode{
				Node: &Node{
					ID:         types.NodeID(fmt.Sprintf("11111111-2222-3333-4444-555555555555-%d", i)),
					Node:       fmt.Sprintf("somehost%05d.local", i),
					Address:    fmt.Sprintf("10.%d.%d.%d", (uint32(i)>>16)&0xff, (uint32(i)>>8)&0xff, uint32(i)&0xff),
					Datacenter: "us-east-1",
				},
				Service: &NodeService{
					ID:      "service-a",
					Service: "service-a",
					Port:    8080,
				},
				Checks: []*HealthCheck{
					&HealthCheck{
						Node:    fmt.Sprintf("somehost%05d.local", i),
						CheckID: "serf",
						Name:    "serf-health",
						Status:  "PASSING",
					},
					&HealthCheck{
						Node:        fmt.Sprintf("somehost%05d.local", i),
						CheckID:     "service-a-http",
						Name:        "service-a-http",
						Status:      "PASSING",
						ServiceID:   "service-a",
						ServiceName: "service-a",
					},
				},
			},
		},
	}
}

func makeSnap(n int) []Event {
	snap := make([]Event, n)
	for j := 0; j < n; j++ {
		snap[j] = Event{
			Index:   uint64(1234567 + j),
			Topic:   Topic_ServiceHealth,
			Key:     "service-a",
			Payload: dummyService(j),
		}
	}
	return snap
}

func doBenchmarkHealthSnapshotUncompressed(b *testing.B, n int) {
	snap := makeSnap(n)
	//lenSample := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Serialize snapshot into protobuf messages
		bytes := 0
		for _, e := range snap {
			buf, err := e.MarshalBinary()
			if err != nil {
				panic(err)
			}
			bytes += len(buf)
		}
		//lenSample = bytes
	}
	//b.Logf("Snapshot size %d bytes", lenSample)
}

func BenchmarkHealthSnapshotUncompressed10(b *testing.B) {
	doBenchmarkHealthSnapshotUncompressed(b, 10)
}
func BenchmarkHealthSnapshotUncompressed1000(b *testing.B) {
	doBenchmarkHealthSnapshotUncompressed(b, 1000)
}
func BenchmarkHealthSnapshotUncompressed10000(b *testing.B) {
	doBenchmarkHealthSnapshotUncompressed(b, 10000)
}

func doBenchmarkHealthSnapshotSnappy(b *testing.B, n int) {
	snap := makeSnap(n)
	lenSample := 0
	lenUncompressedSample := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Serialize snapshot into protobuf messages
		var compressed bytes.Buffer
		compressor := snappy.NewBufferedWriter(&compressed)
		bs := 0

		for _, e := range snap {
			buf, err := e.MarshalBinary()
			if err != nil {
				panic(err)
			}
			compressor.Write(buf)
			bs += len(buf)
		}
		compressor.Close()
		lenSample = compressed.Len()
		lenUncompressedSample = bs
	}
	b.Logf("Snapshot size %d bytes (%d uncompressed)", lenSample, lenUncompressedSample)
}

func BenchmarkHealthSnapshotSnappy10(b *testing.B) {
	doBenchmarkHealthSnapshotSnappy(b, 10)
}
func BenchmarkHealthSnapshotSnappy1000(b *testing.B) {
	doBenchmarkHealthSnapshotSnappy(b, 1000)
}
func BenchmarkHealthSnapshotSnappy10000(b *testing.B) {
	doBenchmarkHealthSnapshotSnappy(b, 10000)
}
