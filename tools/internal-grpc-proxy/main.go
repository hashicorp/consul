package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

// gRPC byte prefix (see RPCGRPC in agent/pool/conn.go).
const bytePrefix byte = 8

func main() {
	if len(os.Args) != 2 {
		log("usage: %s host:port", os.Args[0])
		os.Exit(1)
	}
	serverAddr := os.Args[1]

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log("failed to start listener: %v", err)
		os.Exit(1)
	}
	defer lis.Close()

	fmt.Println("Proxying connections to Consul's internal gRPC server")
	fmt.Printf("Use this address: %s\n", lis.Addr())

	for {
		conn, err := lis.Accept()
		if err != nil {
			log("failed to accept connection: %v", err)
			continue
		}

		go func(conn net.Conn) {
			if err := handleClient(serverAddr, conn); err != nil {
				log(err.Error())
			}
		}(conn)
	}
}

func handleClient(serverAddr string, clientConn net.Conn) error {
	defer clientConn.Close()

	serverConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("failed to dial server connection: %w", err)
	}
	defer serverConn.Close()

	if _, err := serverConn.Write([]byte{bytePrefix}); err != nil {
		return fmt.Errorf("failed to write byte prefix: %v", err)
	}

	errCh := make(chan error, 1)

	go func() {
		_, err := io.Copy(serverConn, clientConn)
		errCh <- err
	}()

	go func() {
		_, err := io.Copy(clientConn, serverConn)
		errCh <- err
	}()

	return <-errCh
}

func log(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message+"\n", args...)
}
