package porter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

var (
	// DefaultAddr is the the default bind address of a Porter server. This acts
	// as the fallback address if the Porter server is not specified.
	DefaultAddr = "127.0.0.1:7965"
)

const (
	// porterErrPrefix is the string returned when displaying a porter error
	porterErrPrefix = `Are you running porter?
Install with 'go install github.com/hashicorp/consul/test/porter/cmd/porter'
Then run 'porter go test ...'`
)

// PorterExistErr is used to wrap an error that is likely from Porter not being
// run.
type PorterExistErr struct {
	Wrapped error
}

func (p *PorterExistErr) Error() string {
	return fmt.Sprintf("%s:\n%s", porterErrPrefix, p.Wrapped)
}

func RandomPorts(n int) ([]int, error) {
	addr := os.Getenv("PORTER_ADDR")
	if addr == "" {
		b, err := ioutil.ReadFile("/tmp/porter.addr")
		if err == nil {
			addr = string(b)
		}
	}
	if addr == "" {
		addr = DefaultAddr
	}
	resp, err := http.Get(fmt.Sprintf("http://%s/%d", addr, n))
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, &PorterExistErr{Wrapped: err}
		}
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var p []int
	err = json.Unmarshal(data, &p)
	return p, err
}
