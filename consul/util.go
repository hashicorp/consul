package consul

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/hashicorp/serf/serf"
)

// ensurePath is used to make sure a path exists
func ensurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

// CanServersUnderstandProtocol checks to see if all the servers in the given
// list understand the given protocol version. If there are no servers in the
// list then this will return false.
func CanServersUnderstandProtocol(members []serf.Member, version uint8) (bool, error) {
	numServers, numWhoGrok := 0, 0
	for _, m := range members {
		if m.Tags["role"] != "consul" {
			continue
		}
		numServers++

		vsn_min, err := strconv.Atoi(m.Tags["vsn_min"])
		if err != nil {
			return false, err
		}

		vsn_max, err := strconv.Atoi(m.Tags["vsn_max"])
		if err != nil {
			return false, err
		}

		v := int(version)
		if (v >= vsn_min) && (v <= vsn_max) {
			numWhoGrok++
		}
	}
	return (numServers > 0) && (numWhoGrok == numServers), nil
}

// Returns if a member is a consul node. Returns a bool,
// and the datacenter.
func isConsulNode(m serf.Member) (bool, string) {
	if m.Tags["role"] != "node" {
		return false, ""
	}
	return true, m.Tags["dc"]
}

// runtimeStats is used to return various runtime information
func runtimeStats() map[string]string {
	return map[string]string{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"version":    runtime.Version(),
		"max_procs":  strconv.FormatInt(int64(runtime.GOMAXPROCS(0)), 10),
		"goroutines": strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		"cpu_count":  strconv.FormatInt(int64(runtime.NumCPU()), 10),
	}
}
