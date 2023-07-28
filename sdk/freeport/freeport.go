// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package freeport provides a helper for reserving free TCP ports across multiple
// processes on the same machine. Each process reserves a block of ports outside
// the ephemeral port range. Tests can request one of these reserved ports
// and freeport will ensure that no other test uses that port until it is returned
// to freeport.
//
// Freeport is particularly useful when the code being tested does not accept
// a net.Listener. Any code that accepts a net.Listener (or uses net/http/httptest.Server)
// can use port 0 (ex: 127.0.0.1:0) to find an unused ephemeral port that will
// not conflict.
//
// Any code that does not accept a net.Listener or can not bind directly to port
// zero should use freeport to find an unused port.
package freeport

import (
	"container/list"
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
)

const (
	// maxBlocks is the number of available port blocks before exclusions.
	maxBlocks = 30

	// lowPort is the lowest port number that should be used.
	lowPort = 10000

	// attempts is how often we try to allocate a port block
	// before giving up.
	attempts = 100
)

var (
	// blockSize is the size of the allocated port block. ports are given out
	// consecutively from that block and after that point in a LRU fashion.
	blockSize int

	// effectiveMaxBlocks is the number of available port blocks.
	// lowPort + effectiveMaxBlocks * blockSize must be less than 65535.
	effectiveMaxBlocks int

	// firstPort is the first port of the allocated block.
	firstPort int

	// lockLn is the system-wide mutex for the port block.
	lockLn net.Listener

	// mu guards:
	// - pendingPorts
	// - freePorts
	// - total
	mu sync.Mutex

	// once is used to do the initialization on the first call to retrieve free
	// ports
	once sync.Once

	// condNotEmpty is a condition variable to wait for freePorts to be not
	// empty. Linked to 'mu'
	condNotEmpty *sync.Cond

	// freePorts is a FIFO of all currently free ports. Take from the front,
	// and return to the back.
	freePorts *list.List

	// pendingPorts is a FIFO of recently freed ports that have not yet passed
	// the not-in-use check.
	pendingPorts *list.List

	// total is the total number of available ports in the block for use.
	total int

	// seededRand is a random generator that is pre-seeded from the current time.
	seededRand *rand.Rand

	// stopCh is used to signal to background goroutines to terminate. Only
	// really exists for the safety of reset() during unit tests.
	stopCh chan struct{}

	// stopWg is used to keep track of background goroutines that are still
	// alive. Only really exists for the safety of reset() during unit tests.
	stopWg sync.WaitGroup

	// portLastUser associates ports with a test name in order to debug
	// which test may be leaking unclosed TCP connections.
	portLastUser map[int]string
)

// initialize is used to initialize freeport.
func initialize() {
	var err error

	blockSize = 1500
	limit, err := systemLimit()
	if err != nil {
		panic("freeport: error getting system limit: " + err.Error())
	}
	if limit > 0 && limit < blockSize {
		logf("INFO", "blockSize %d too big for system limit %d. Adjusting...", blockSize, limit)
		blockSize = limit - 3
	}

	effectiveMaxBlocks, err = adjustMaxBlocks()
	if err != nil {
		panic("freeport: ephemeral port range detection failed: " + err.Error())
	}
	if effectiveMaxBlocks < 0 {
		panic("freeport: no blocks of ports available outside of ephemeral range")
	}
	if lowPort+effectiveMaxBlocks*blockSize > 65535 {
		panic("freeport: block size too big or too many blocks requested")
	}

	seededRand = rand.New(rand.NewSource(time.Now().UnixNano())) // This is compatible with go 1.19 but unnecessary in >= go1.20
	firstPort, lockLn = alloc()

	condNotEmpty = sync.NewCond(&mu)
	freePorts = list.New()
	pendingPorts = list.New()

	// fill with all available free ports
	for port := firstPort + 1; port < firstPort+blockSize; port++ {
		if used := isPortInUse(port); !used {
			freePorts.PushBack(port)
		}
	}
	total = freePorts.Len()

	stopWg.Add(1)
	stopCh = make(chan struct{})

	portLastUser = make(map[int]string)
	// Note: we pass this param explicitly to the goroutine so that we can
	// freely recreate the underlying stop channel during reset() after closing
	// the original.
	go checkFreedPorts(stopCh)
}

func shutdownGoroutine() {
	mu.Lock()
	if stopCh == nil {
		mu.Unlock()
		return
	}

	close(stopCh)
	stopCh = nil
	mu.Unlock()

	stopWg.Wait()
}

// reset will reverse the setup from initialize() and then redo it (for tests)
func reset() {
	logf("INFO", "resetting the freeport package state")
	shutdownGoroutine()

	mu.Lock()
	defer mu.Unlock()

	effectiveMaxBlocks = 0
	firstPort = 0
	if lockLn != nil {
		lockLn.Close()
		lockLn = nil
	}

	once = sync.Once{}

	freePorts = nil
	pendingPorts = nil
	portLastUser = nil
	total = 0
}

func checkFreedPorts(stopCh <-chan struct{}) {
	defer stopWg.Done()

	ticker := time.NewTicker(250 * time.Millisecond)
	for {
		select {
		case <-stopCh:
			logf("INFO", "Closing checkFreedPorts()")
			return
		case <-ticker.C:
			checkFreedPortsOnce()
		}
	}
}

func checkFreedPortsOnce() {
	mu.Lock()
	defer mu.Unlock()

	pending := pendingPorts.Len()
	remove := make([]*list.Element, 0, pending)
	for elem := pendingPorts.Front(); elem != nil; elem = elem.Next() {
		port := elem.Value.(int)
		if used := isPortInUse(port); !used {
			freePorts.PushBack(port)
			remove = append(remove, elem)
		} else {
			logf("WARN", "port %d still being used by %q", port, portLastUser[port])
		}
	}

	retained := pending - len(remove)

	if retained > 0 {
		logf("WARN", "%d out of %d pending ports are still in use; something probably didn't wait around for the port to be closed!", retained, pending)
	}

	if len(remove) == 0 {
		return
	}

	for _, elem := range remove {
		pendingPorts.Remove(elem)
	}

	condNotEmpty.Broadcast()
}

// adjustMaxBlocks avoids having the allocation ranges overlap the ephemeral
// port range.
func adjustMaxBlocks() (int, error) {
	ephemeralPortMin, ephemeralPortMax, err := getEphemeralPortRange()
	if err != nil {
		return 0, err
	}

	if ephemeralPortMin <= 0 || ephemeralPortMax <= 0 {
		logf("INFO", "ephemeral port range detection not configured for GOOS=%q", runtime.GOOS)
		return maxBlocks, nil
	}

	logf("INFO", "detected ephemeral port range of [%d, %d]", ephemeralPortMin, ephemeralPortMax)
	for block := 0; block < maxBlocks; block++ {
		min := lowPort + block*blockSize
		max := min + blockSize
		overlap := intervalOverlap(min, max-1, ephemeralPortMin, ephemeralPortMax)
		if overlap {
			logf("INFO", "reducing max blocks from %d to %d to avoid the ephemeral port range", maxBlocks, block)
			return block, nil
		}
	}
	return maxBlocks, nil
}

// alloc reserves a port block for exclusive use for the lifetime of the
// application. lockLn serves as a system-wide mutex for the port block and is
// implemented as a TCP listener which is bound to the firstPort and which will
// be automatically released when the application terminates.
func alloc() (int, net.Listener) {
	var err error
	// note: can't use retry here because we don't have a Failer (testing.T)
	for i := 0; i < attempts; i++ {
		block := int(seededRand.Int31n(int32(effectiveMaxBlocks)))
		firstPort := lowPort + block*blockSize
		var ln *net.TCPListener
		ln, err = net.ListenTCP("tcp", tcpAddr("127.0.0.1", firstPort))
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		// logf("DEBUG", "allocated port block %d (%d-%d)", block, firstPort, firstPort+blockSize-1)
		return firstPort, ln
	}
	panic("freeport: cannot allocate port block: " + err.Error())
}

// MustTake is the same as Take except it panics on error.
//
// Deprecated: Use GetN or GetOne instead.
func MustTake(n int) (ports []int) {
	ports, err := Take(n)
	if err != nil {
		panic(err)
	}
	return ports
}

// Take returns a list of free ports from the reserved port block. It is safe
// to call this method concurrently. Ports have been tested to be available on
// 127.0.0.1 TCP but there is no guarantee that they will remain free in the
// future.
//
// Most callers should prefer GetN or GetOne.
func Take(n int) (ports []int, err error) {
	if n <= 0 {
		return nil, fmt.Errorf("freeport: cannot take %d ports", n)
	}

	mu.Lock()
	defer mu.Unlock()

	// Reserve a port block
	once.Do(initialize)

	if n > total {
		return nil, fmt.Errorf("freeport: block size too small")
	}

	for len(ports) < n {
		for freePorts.Len() == 0 {
			if total == 0 {
				return nil, fmt.Errorf("freeport: impossible to satisfy request; there are no actual free ports in the block anymore")
			}
			condNotEmpty.Wait()
		}

		elem := freePorts.Front()
		freePorts.Remove(elem)
		port := elem.Value.(int)

		if used := isPortInUse(port); used {
			// Something outside of the test suite has stolen this port, possibly
			// due to assignment to an ephemeral port, remove it completely.
			logf("WARN", "leaked port %d due to theft; removing from circulation", port)
			total--
			continue
		}

		ports = append(ports, port)
	}

	return ports, nil
}

// peekFree returns the next port that will be returned by Take to aid in testing.
func peekFree() int {
	mu.Lock()
	defer mu.Unlock()
	return freePorts.Front().Value.(int)
}

// peekAllFree returns all free ports that could be returned by Take to aid in testing.
func peekAllFree() []int {
	mu.Lock()
	defer mu.Unlock()

	var out []int
	for elem := freePorts.Front(); elem != nil; elem = elem.Next() {
		port := elem.Value.(int)
		out = append(out, port)
	}

	return out
}

// stats returns diagnostic data to aid in testing
func stats() (numTotal, numPending, numFree int) {
	mu.Lock()
	defer mu.Unlock()
	return total, pendingPorts.Len(), freePorts.Len()
}

// Return returns a block of ports back to the general pool. These ports should
// have been returned from a call to Take().
func Return(ports []int) {
	if len(ports) == 0 {
		return // convenience short circuit for test ergonomics
	}

	mu.Lock()
	defer mu.Unlock()

	for _, port := range ports {
		if port > firstPort && port < firstPort+blockSize {
			pendingPorts.PushBack(port)
		}
	}
}

func isPortInUse(port int) bool {
	ln, err := net.ListenTCP("tcp", tcpAddr("127.0.0.1", port))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

func tcpAddr(ip string, port int) *net.TCPAddr {
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
}

// intervalOverlap returns true if the doubly-inclusive integer intervals
// represented by [min1, max1] and [min2, max2] overlap.
func intervalOverlap(min1, max1, min2, max2 int) bool {
	if min1 > max1 {
		logf("WARN", "interval1 is not ordered [%d, %d]", min1, max1)
		return false
	}
	if min2 > max2 {
		logf("WARN", "interval2 is not ordered [%d, %d]", min2, max2)
		return false
	}
	return min1 <= max2 && min2 <= max1
}

func logf(severity string, format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "["+severity+"] freeport: "+format+"\n", a...)
}

// TestingT is the minimal set of methods implemented by *testing.T that are
// used by functions in freelist.
//
// In the future new methods may be added to this interface, but those methods
// should always be implemented by *testing.T
type TestingT interface {
	Cleanup(func())
	Helper()
	Fatalf(format string, args ...interface{})
	Name() string
	FailNow()
	Log(args ...interface{})
}

func mayGetN(t TestingT, n int) ([]int, error) {
	t.Helper()
	ports, err := Take(n)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		Return(ports)
		logf("DEBUG", "Test %q returned ports %v", t.Name(), ports)
	})
	logf("DEBUG", "Test %q took ports %v", t.Name(), ports)
	mu.Lock()
	for _, p := range ports {
		portLastUser[p] = t.Name()
	}
	mu.Unlock()
	return ports, nil
}

// GetN returns n free ports from the reserved port block, and returns the
// ports to the pool when the test ends. See Take for more details.
//
// TODO: rename to MustGetN
func GetN(t TestingT, n int) []int {
	t.Helper()
	ports, err := mayGetN(t, n)
	if err != nil {
		t.Fatalf("failed to take %v ports: %w", n, err)
	}
	return ports
}

// RetryMustGetN is like MustGetN, but it will retry using retryer
func RetryMustGetN(t TestingT, retryer retry.Retryer, n int) []int {
	t.Helper()
	var ports []int
	retry.RunWith(retryer, t, func(r *retry.R) {
		var err error
		ports, err = mayGetN(t, n)
		require.NoError(r, err)
	})
	return ports
}

// GetOne returns a single free port from the reserved port block, and returns the
// port to the pool when the test ends. See Take for more details.
// Use GetN if more than a single port is required.
func GetOne(t TestingT) int {
	t.Helper()
	return GetN(t, 1)[0]
}

// Deprecated: Please use Take/Return calls instead.
func Get(n int) (ports []int) { return MustTake(n) }

// Deprecated: Please use Take/Return calls instead.
func GetT(t TestingT, n int) (ports []int) { return MustTake(n) }

// Deprecated: Please use Take/Return calls instead.
func Free(n int) (ports []int, err error) { return MustTake(n), nil }
