// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ports

import (
	"fmt"
	"net"
	"time"
)

type troubleShootTcp struct {
}

func (tcp *troubleShootTcp) dailPort(hostPort *hostPort) string {
	address := net.JoinHostPort(hostPort.host, hostPort.port)

	// Attempt to establish a TCP connection with a timeout.
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Sprintf("TCP: Port %s on %s is closed, unreachable, or the connection timed out.\n", hostPort.port, hostPort.host)
	}
	defer conn.Close()

	// If no error occurs, the connection was successful, and the port is open.
	return fmt.Sprintf("TCP: Port %s on %s is open.\n", hostPort.port, hostPort.host)
}
