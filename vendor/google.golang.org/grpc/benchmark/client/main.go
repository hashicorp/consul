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
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/benchmark"
	testpb "google.golang.org/grpc/benchmark/grpc_testing"
	"google.golang.org/grpc/benchmark/stats"
	"google.golang.org/grpc/grpclog"
)

var (
	port      = flag.String("port", "50051", "Localhost port to connect to.")
	numRPC    = flag.Int("r", 1, "The number of concurrent RPCs on each connection.")
	numConn   = flag.Int("c", 1, "The number of parallel connections.")
	warmupDur = flag.Int("w", 10, "Warm-up duration in seconds")
	duration  = flag.Int("d", 60, "Benchmark duration in seconds")
	rqSize    = flag.Int("req", 1, "Request message size in bytes.")
	rspSize   = flag.Int("resp", 1, "Response message size in bytes.")
	rpcType   = flag.String("rpc_type", "unary",
		`Configure different client rpc type. Valid options are:
		   unary;
		   streaming.`)
	testName = flag.String("test_name", "", "Name of the test used for creating profiles.")
	wg       sync.WaitGroup
	hopts    = stats.HistogramOptions{
		NumBuckets:   2495,
		GrowthFactor: .01,
	}
	mu    sync.Mutex
	hists []*stats.Histogram
)

func main() {
	flag.Parse()
	if *testName == "" {
		grpclog.Fatalf("test_name not set")
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(*rspSize),
		Payload: &testpb.Payload{
			Type: testpb.PayloadType_COMPRESSABLE,
			Body: make([]byte, *rqSize),
		},
	}
	connectCtx, connectCancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer connectCancel()
	ccs := buildConnections(connectCtx)
	warmDeadline := time.Now().Add(time.Duration(*warmupDur) * time.Second)
	endDeadline := warmDeadline.Add(time.Duration(*duration) * time.Second)
	cf, err := os.Create("/tmp/" + *testName + ".cpu")
	if err != nil {
		grpclog.Fatalf("Error creating file: %v", err)
	}
	defer cf.Close()
	pprof.StartCPUProfile(cf)
	cpuBeg := getCPUTime()
	for _, cc := range ccs {
		runWithConn(cc, req, warmDeadline, endDeadline)
	}
	wg.Wait()
	cpu := time.Duration(getCPUTime() - cpuBeg)
	pprof.StopCPUProfile()
	mf, err := os.Create("/tmp/" + *testName + ".mem")
	if err != nil {
		grpclog.Fatalf("Error creating file: %v", err)
	}
	defer mf.Close()
	runtime.GC() // materialize all statistics
	if err := pprof.WriteHeapProfile(mf); err != nil {
		grpclog.Fatalf("Error writing memory profile: %v", err)
	}
	hist := stats.NewHistogram(hopts)
	for _, h := range hists {
		hist.Merge(h)
	}
	parseHist(hist)
	fmt.Println("Client CPU utilization:", cpu)
	fmt.Println("Client CPU profile:", cf.Name())
	fmt.Println("Client Mem Profile:", mf.Name())
}

func buildConnections(ctx context.Context) []*grpc.ClientConn {
	ccs := make([]*grpc.ClientConn, *numConn)
	for i := range ccs {
		ccs[i] = benchmark.NewClientConnWithContext(ctx, "localhost:"+*port, grpc.WithInsecure(), grpc.WithBlock())
	}
	return ccs
}

func runWithConn(cc *grpc.ClientConn, req *testpb.SimpleRequest, warmDeadline, endDeadline time.Time) {
	for i := 0; i < *numRPC; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			caller := makeCaller(cc, req)
			hist := stats.NewHistogram(hopts)
			for {
				start := time.Now()
				if start.After(endDeadline) {
					mu.Lock()
					hists = append(hists, hist)
					mu.Unlock()
					return
				}
				caller()
				elapsed := time.Since(start)
				if start.After(warmDeadline) {
					hist.Add(elapsed.Nanoseconds())
				}
			}
		}()
	}
}

func makeCaller(cc *grpc.ClientConn, req *testpb.SimpleRequest) func() {
	client := testpb.NewBenchmarkServiceClient(cc)
	if *rpcType == "unary" {
		return func() {
			if _, err := client.UnaryCall(context.Background(), req); err != nil {
				grpclog.Fatalf("RPC failed: %v", err)
			}
		}
	}
	stream, err := client.StreamingCall(context.Background())
	if err != nil {
		grpclog.Fatalf("RPC failed: %v", err)
	}
	return func() {
		if err := stream.Send(req); err != nil {
			grpclog.Fatalf("Streaming RPC failed to send: %v", err)
		}
		if _, err := stream.Recv(); err != nil {
			grpclog.Fatalf("Streaming RPC failed to read: %v", err)
		}
	}
}

func parseHist(hist *stats.Histogram) {
	fmt.Println("qps:", float64(hist.Count)/float64(*duration))
	fmt.Printf("Latency: (50/90/99 %%ile): %v/%v/%v\n",
		time.Duration(median(.5, hist)),
		time.Duration(median(.9, hist)),
		time.Duration(median(.99, hist)))
}

func median(percentile float64, h *stats.Histogram) int64 {
	need := int64(float64(h.Count) * percentile)
	have := int64(0)
	for _, bucket := range h.Buckets {
		count := bucket.Count
		if have+count >= need {
			percent := float64(need-have) / float64(count)
			return int64((1.0-percent)*bucket.LowBound + percent*bucket.LowBound*(1.0+hopts.GrowthFactor))
		}
		have += bucket.Count
	}
	panic("should have found a bound")
}

func getCPUTime() int64 {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_PROCESS_CPUTIME_ID, &ts); err != nil {
		grpclog.Fatal(err)
	}
	return ts.Nano()
}
