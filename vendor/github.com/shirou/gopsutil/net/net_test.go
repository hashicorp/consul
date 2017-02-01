package net

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/internal/common"
)

func TestAddrString(t *testing.T) {
	v := Addr{IP: "192.168.0.1", Port: 8000}

	s := fmt.Sprintf("%v", v)
	if s != "{\"ip\":\"192.168.0.1\",\"port\":8000}" {
		t.Errorf("Addr string is invalid: %v", v)
	}
}

func TestNetIOCountersStatString(t *testing.T) {
	v := IOCountersStat{
		Name:      "test",
		BytesSent: 100,
	}
	e := `{"name":"test","bytesSent":100,"bytesRecv":0,"packetsSent":0,"packetsRecv":0,"errin":0,"errout":0,"dropin":0,"dropout":0,"fifoin":0,"fifoout":0}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("NetIOCountersStat string is invalid: %v", v)
	}
}

func TestNetProtoCountersStatString(t *testing.T) {
	v := ProtoCountersStat{
		Protocol: "tcp",
		Stats: map[string]int64{
			"MaxConn":      -1,
			"ActiveOpens":  4000,
			"PassiveOpens": 3000,
		},
	}
	e := `{"protocol":"tcp","stats":{"ActiveOpens":4000,"MaxConn":-1,"PassiveOpens":3000}}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("NetProtoCountersStat string is invalid: %v", v)
	}

}

func TestNetConnectionStatString(t *testing.T) {
	v := ConnectionStat{
		Fd:     10,
		Family: 10,
		Type:   10,
		Uids:   []int32{10, 10},
	}
	e := `{"fd":10,"family":10,"type":10,"localaddr":{"ip":"","port":0},"remoteaddr":{"ip":"","port":0},"status":"","uids":[10,10],"pid":0}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("NetConnectionStat string is invalid: %v", v)
	}

}

func TestNetIOCountersAll(t *testing.T) {
	v, err := IOCounters(false)
	per, err := IOCounters(true)
	if err != nil {
		t.Errorf("Could not get NetIOCounters: %v", err)
	}
	if len(v) != 1 {
		t.Errorf("Could not get NetIOCounters: %v", v)
	}
	if v[0].Name != "all" {
		t.Errorf("Invalid NetIOCounters: %v", v)
	}
	var pr uint64
	for _, p := range per {
		pr += p.PacketsRecv
	}
	if v[0].PacketsRecv != pr {
		t.Errorf("invalid sum value: %v, %v", v[0].PacketsRecv, pr)
	}
}

func TestNetIOCountersPerNic(t *testing.T) {
	v, err := IOCounters(true)
	if err != nil {
		t.Errorf("Could not get NetIOCounters: %v", err)
	}
	if len(v) == 0 {
		t.Errorf("Could not get NetIOCounters: %v", v)
	}
	for _, vv := range v {
		if vv.Name == "" {
			t.Errorf("Invalid NetIOCounters: %v", vv)
		}
	}
}

func TestGetNetIOCountersAll(t *testing.T) {
	n := []IOCountersStat{
		IOCountersStat{
			Name:        "a",
			BytesRecv:   10,
			PacketsRecv: 10,
		},
		IOCountersStat{
			Name:        "b",
			BytesRecv:   10,
			PacketsRecv: 10,
			Errin:       10,
		},
	}
	ret, err := getIOCountersAll(n)
	if err != nil {
		t.Error(err)
	}
	if len(ret) != 1 {
		t.Errorf("invalid return count")
	}
	if ret[0].Name != "all" {
		t.Errorf("invalid return name")
	}
	if ret[0].BytesRecv != 20 {
		t.Errorf("invalid count bytesrecv")
	}
	if ret[0].Errin != 10 {
		t.Errorf("invalid count errin")
	}
}

func TestNetInterfaces(t *testing.T) {
	v, err := Interfaces()
	if err != nil {
		t.Errorf("Could not get NetInterfaceStat: %v", err)
	}
	if len(v) == 0 {
		t.Errorf("Could not get NetInterfaceStat: %v", err)
	}
	for _, vv := range v {
		if vv.Name == "" {
			t.Errorf("Invalid NetInterface: %v", vv)
		}
	}
}

func TestNetProtoCountersStatsAll(t *testing.T) {
	v, err := ProtoCounters(nil)
	if err != nil {
		t.Fatalf("Could not get NetProtoCounters: %v", err)
	}
	if len(v) == 0 {
		t.Fatalf("Could not get NetProtoCounters: %v", err)
	}
	for _, vv := range v {
		if vv.Protocol == "" {
			t.Errorf("Invalid NetProtoCountersStat: %v", vv)
		}
		if len(vv.Stats) == 0 {
			t.Errorf("Invalid NetProtoCountersStat: %v", vv)
		}
	}
}

func TestNetProtoCountersStats(t *testing.T) {
	v, err := ProtoCounters([]string{"tcp", "ip"})
	if err != nil {
		t.Fatalf("Could not get NetProtoCounters: %v", err)
	}
	if len(v) == 0 {
		t.Fatalf("Could not get NetProtoCounters: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("Go incorrect number of NetProtoCounters: %v", err)
	}
	for _, vv := range v {
		if vv.Protocol != "tcp" && vv.Protocol != "ip" {
			t.Errorf("Invalid NetProtoCountersStat: %v", vv)
		}
		if len(vv.Stats) == 0 {
			t.Errorf("Invalid NetProtoCountersStat: %v", vv)
		}
	}
}

func TestNetConnections(t *testing.T) {
	if ci := os.Getenv("CI"); ci != "" { // skip if test on drone.io
		return
	}

	v, err := Connections("inet")
	if err != nil {
		t.Errorf("could not get NetConnections: %v", err)
	}
	if len(v) == 0 {
		t.Errorf("could not get NetConnections: %v", v)
	}
	for _, vv := range v {
		if vv.Family == 0 {
			t.Errorf("invalid NetConnections: %v", vv)
		}
	}

}

func TestNetFilterCounters(t *testing.T) {
	if ci := os.Getenv("CI"); ci != "" { // skip if test on drone.io
		return
	}

	if runtime.GOOS == "linux" {
		// some test environment has not the path.
		if !common.PathExists("/proc/sys/net/netfilter/nf_conntrackCount") {
			t.SkipNow()
		}
	}

	v, err := FilterCounters()
	if err != nil {
		t.Errorf("could not get NetConnections: %v", err)
	}
	if len(v) == 0 {
		t.Errorf("could not get NetConnections: %v", v)
	}
	for _, vv := range v {
		if vv.ConnTrackMax == 0 {
			t.Errorf("nf_conntrackMax needs to be greater than zero: %v", vv)
		}
	}

}
