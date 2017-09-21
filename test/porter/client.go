package porter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

var DefaultAddr = "127.0.0.1:7965"

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
