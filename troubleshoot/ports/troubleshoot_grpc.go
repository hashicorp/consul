package ports

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"net"
	"time"
)

const GRPC_PROTOCOL = "grpc"

type TroubleShootGrpc struct {
}

func (*TroubleShootGrpc) test(hostPort *HostPort, ch chan string) {
	timeout := 5 * time.Second

	// Combine the host and port to form the address.
	address := net.JoinHostPort(hostPort.host, hostPort.port)

	// Set up a context with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Attempt to establish a gRPC connection with a timeout.
	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure())
	if err != nil {
		// If an error occurs, the gRPC port is likely closed, unreachable, or the connection timed out.
		ch <- fmt.Sprintf("GRPC: Port %s on %s is closed, unreachable, or the connection timed out.\n", hostPort.port, hostPort.host)
		return
	}
	defer conn.Close()

	// If no error occurs, the connection was successful, and the gRPC port is open.
	ch <- fmt.Sprintf("GRPC: Port %s on %s is open.\n", hostPort.port, hostPort.host)
}
