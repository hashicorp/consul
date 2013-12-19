package consul

import (
	"github.com/hashicorp/serf/serf"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// strContains checks if a list contains a string
func strContains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

// ensurePath is used to make sure a path exists
func ensurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

// Returns if a member is a consul server. Returns a bool,
// the data center, and the rpc port
func isConsulServer(m serf.Member) (bool, string, int) {
	role := m.Role
	if !strings.HasPrefix(role, "consul:") {
		return false, "", 0
	}

	parts := strings.SplitN(role, ":", 3)
	datacenter := parts[1]
	port_str := parts[2]
	port, err := strconv.Atoi(port_str)
	if err != nil {
		return false, "", 0
	}

	return true, datacenter, port
}
