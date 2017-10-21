// Package freeport provides a helper for allocating free ports across multiple
// processes on the same machine.
package freeport

import (
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	// blockSize is the size of the allocated port block. ports are given out
	// consecutively from that block with roll-over for the lifetime of the
	// application/test run.
	blockSize = 500

	// maxBlocks is the number of available port blocks.
	// lowPort + maxBlocks * blockSize must be less than 65535.
	maxBlocks = 30

	// lowPort is the lowest port number that should be used.
	lowPort = 10000

	// attempts is how often we try to allocate a port block
	// before giving up.
	attempts = 10
)

var (
	// firstPort is the first port of the allocated block.
	firstPort int

	// lockLn is the system-wide mutex for the port block.
	lockLn net.Listener

	// mu guards nextPort
	mu sync.Mutex

	// port is the last allocated port.
	port int
)

func init() {
	if lowPort+maxBlocks*blockSize > 65535 {
		panic("freeport: block size too big or too many blocks requested")
	}

	rand.Seed(time.Now().UnixNano())
	firstPort, lockLn = alloc()
}

// alloc reserves a port block for exclusive use for the lifetime of the
// application. lockLn serves as a system-wide mutex for the port block and is
// implemented as a TCP listener which is bound to the firstPort and which will
// be automatically released when the application terminates.
func alloc() (int, net.Listener) {
	for i := 0; i < attempts; i++ {
		block := int(rand.Int31n(int32(maxBlocks)))
		firstPort := lowPort + block*blockSize
		ln, err := net.ListenTCP("tcp", tcpAddr("127.0.0.1", firstPort))
		if err != nil {
			continue
		}
		// log.Printf("[DEBUG] freeport: allocated port block %d (%d-%d)", block, firstPort, firstPort+blockSize-1)
		return firstPort, ln
	}
	panic("freeport: cannot allocate port block")
}

func tcpAddr(ip string, port int) *net.TCPAddr {
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
}

// Get returns a list of free ports from the allocated port block. It is safe
// to call this method concurrently. Ports have been tested to be available on
// 127.0.0.1 TCP but there is no guarantee that they will remain free in the
// future.
func Get(n int) (ports []int) {
	mu.Lock()
	defer mu.Unlock()

	if n > blockSize-1 {
		panic("freeport: block size too small")
	}

	for len(ports) < n {
		port++

		// roll-over the port
		if port < firstPort+1 || port >= firstPort+blockSize {
			port = firstPort + 1
		}

		// if the port is in use then skip it
		ln, err := net.ListenTCP("tcp", tcpAddr("127.0.0.1", port))
		if err != nil {
			// log.Println("[DEBUG] freeport: port already in use: ", port)
			continue
		}
		ln.Close()

		ports = append(ports, port)
	}
	// log.Println("[DEBUG] freeport: free ports:", ports)
	return ports
}
