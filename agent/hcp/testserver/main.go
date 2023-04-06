// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/consul/agent/hcp/bootstrap"
)

var port int

func main() {
	flag.IntVar(&port, "port", 9999, "port to listen on")
	flag.Parse()

	s := hcp.NewMockHCPServer()
	s.AddEndpoint(bootstrap.TestEndpoint())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := http.Server{
		Addr:    addr,
		Handler: s,
	}

	log.Printf("Listening on %s\n", addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-sigs
	log.Println("Shutting down HTTP server")
	srv.Close()
}
