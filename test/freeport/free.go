package freeport

import (
	"fmt"
	"net"
	"sync"

	"github.com/mitchellh/go-testing-interface"
)

const (
	// mruSize is the mru size for the handed out ports.
	mruSize = 4096
)

var (
	// Avoid handing out the same ports that we just handed out
	mru       = make([]int, mruSize)
	latestIdx int
	m         sync.Mutex
)

// usePort returns whether the port should be handed out or if the port is
// within our MRU.
func usePort(p int) bool {
	m.Lock()

	// Check if the port is used in the last 10
	for _, used := range mru {
		if used == p {
			m.Unlock()
			return false
		}
	}

	// Add the port to the mru
	mru[latestIdx%mruSize] = p
	latestIdx++
	m.Unlock()
	return true
}

func Port() (int, error) {
	retry := 0

RETRY:
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return -1, err
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return -1, fmt.Errorf("unexpected address type: %T", ln.Addr())
	}

	p := addr.Port
	if !usePort(p) {
		if retry >= 5 {
			return -1, fmt.Errorf("retried and failed retrieving a port %d times", retry)
		}

		retry++
		goto RETRY
	}

	return p, nil
}

func Ports(n int) ([]int, error) {
	ports := make([]int, n)
	for i := 0; i < n; i++ {
		p, err := Port()
		if err != nil {
			return nil, err
		}
		ports[i] = p
	}

	return ports, nil
}

func Get(t testing.T) int {
	p, err := Port()
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}

	return p
}

func GetN(t testing.T, n int) []int {
	ports, err := Ports(n)
	if err != nil {
		t.Fatalf("failed to get free ports: %v", err)
	}

	return ports
}
