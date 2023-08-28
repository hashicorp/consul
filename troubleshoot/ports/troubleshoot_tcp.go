package ports

import (
	"fmt"
	"net"
	"time"
)

type troubleShootTcp struct {
}

func (tcp *troubleShootTcp) dailPort(hostPort *hostPort, ch chan string) {
	address := net.JoinHostPort(hostPort.host, hostPort.port)

	// Attempt to establish a TCP connection with a timeout.
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		ch <- fmt.Sprintf("TCP: Port %s on %s is closed, unreachable, or the connection timed out.\n", hostPort.port, hostPort.host)
		return
	}
	defer conn.Close()

	// If no error occurs, the connection was successful, and the port is open.
	ch <- fmt.Sprintf("TCP: Port %s on %s is open.\n", hostPort.port, hostPort.host)
}
