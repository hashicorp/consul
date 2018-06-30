/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"flag"
	"fmt"
	"net"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/benchmark"
	"google.golang.org/grpc/grpclog"
)

var (
	port     = flag.String("port", "50051", "Localhost port to listen on.")
	testName = flag.String("test_name", "", "Name of the test used for creating profiles.")
)

func main() {
	flag.Parse()
	if *testName == "" {
		grpclog.Fatalf("test name not set")
	}
	lis, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		grpclog.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	cf, err := os.Create("/tmp/" + *testName + ".cpu")
	if err != nil {
		grpclog.Fatalf("Failed to create file: %v", err)
	}
	defer cf.Close()
	pprof.StartCPUProfile(cf)
	cpuBeg := getCPUTime()
	// Launch server in a separate goroutine.
	stop := benchmark.StartServer(benchmark.ServerInfo{Type: "protobuf", Listener: lis})
	// Wait on OS terminate signal.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM)
	<-ch
	cpu := time.Duration(getCPUTime() - cpuBeg)
	stop()
	pprof.StopCPUProfile()
	mf, err := os.Create("/tmp/" + *testName + ".mem")
	if err != nil {
		grpclog.Fatalf("Failed to create file: %v", err)
	}
	defer mf.Close()
	runtime.GC() // materialize all statistics
	if err := pprof.WriteHeapProfile(mf); err != nil {
		grpclog.Fatalf("Failed to write memory profile: %v", err)
	}
	fmt.Println("Server CPU utilization:", cpu)
	fmt.Println("Server CPU profile:", cf.Name())
	fmt.Println("Server Mem Profile:", mf.Name())
}

func getCPUTime() int64 {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_PROCESS_CPUTIME_ID, &ts); err != nil {
		grpclog.Fatal(err)
	}
	return ts.Nano()
}
