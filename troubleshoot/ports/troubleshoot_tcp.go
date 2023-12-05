// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ports

import (
	"net"
	"time"
)

type troubleShootTcp struct {
}

func (tcp *troubleShootTcp) dialPort(hostPort *hostPort) error {
	address := net.JoinHostPort(hostPort.host, hostPort.port)

	// Attempt to establish a TCP connection with a timeout.
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}
