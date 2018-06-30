/*
 *
 * Copyright 2018 gRPC authors.
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

package test

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer/grpclb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/internal/channelz"
	"google.golang.org/grpc/internal/leakcheck"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/status"
	testpb "google.golang.org/grpc/test/grpc_testing"
)

func init() {
	channelz.TurnOn()
}

func (te *test) startServers(ts testpb.TestServiceServer, num int) {
	for i := 0; i < num; i++ {
		te.startServer(ts)
		te.srvs = append(te.srvs, te.srv)
		te.srvAddrs = append(te.srvAddrs, te.srvAddr)
		te.srv = nil
		te.srvAddr = ""
	}
}

func verifyResultWithDelay(f func() (bool, error)) error {
	var ok bool
	var err error
	for i := 0; i < 1000; i++ {
		if ok, err = f(); ok {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

func TestCZServerRegistrationAndDeletion(t *testing.T) {
	defer leakcheck.Check(t)
	testcases := []struct {
		total  int
		start  int64
		length int
		end    bool
	}{
		{total: channelz.EntryPerPage, start: 0, length: channelz.EntryPerPage, end: true},
		{total: channelz.EntryPerPage - 1, start: 0, length: channelz.EntryPerPage - 1, end: true},
		{total: channelz.EntryPerPage + 1, start: 0, length: channelz.EntryPerPage, end: false},
		{total: channelz.EntryPerPage + 1, start: int64(2*(channelz.EntryPerPage+1) + 1), length: 0, end: true},
	}

	for _, c := range testcases {
		channelz.NewChannelzStorage()
		e := tcpClearRREnv
		te := newTest(t, e)
		te.startServers(&testServer{security: e.security}, c.total)

		ss, end := channelz.GetServers(c.start)
		if len(ss) != c.length || end != c.end {
			t.Fatalf("GetServers(%d) = %+v (len of which: %d), end: %+v, want len(GetServers(%d)) = %d, end: %+v", c.start, ss, len(ss), end, c.start, c.length, c.end)
		}
		te.tearDown()
		ss, end = channelz.GetServers(c.start)
		if len(ss) != 0 || !end {
			t.Fatalf("GetServers(0) = %+v (len of which: %d), end: %+v, want len(GetServers(0)) = 0, end: true", ss, len(ss), end)
		}
	}
}

func TestCZTopChannelRegistrationAndDeletion(t *testing.T) {
	defer leakcheck.Check(t)
	testcases := []struct {
		total  int
		start  int64
		length int
		end    bool
	}{
		{total: channelz.EntryPerPage, start: 0, length: channelz.EntryPerPage, end: true},
		{total: channelz.EntryPerPage - 1, start: 0, length: channelz.EntryPerPage - 1, end: true},
		{total: channelz.EntryPerPage + 1, start: 0, length: channelz.EntryPerPage, end: false},
		{total: channelz.EntryPerPage + 1, start: int64(2*(channelz.EntryPerPage+1) + 1), length: 0, end: true},
	}

	for _, c := range testcases {
		channelz.NewChannelzStorage()
		e := tcpClearRREnv
		te := newTest(t, e)
		var ccs []*grpc.ClientConn
		for i := 0; i < c.total; i++ {
			cc := te.clientConn()
			te.cc = nil
			// avoid making next dial blocking
			te.srvAddr = ""
			ccs = append(ccs, cc)
		}
		if err := verifyResultWithDelay(func() (bool, error) {
			if tcs, end := channelz.GetTopChannels(c.start); len(tcs) != c.length || end != c.end {
				return false, fmt.Errorf("GetTopChannels(%d) = %+v (len of which: %d), end: %+v, want len(GetTopChannels(%d)) = %d, end: %+v", c.start, tcs, len(tcs), end, c.start, c.length, c.end)
			}
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}

		for _, cc := range ccs {
			cc.Close()
		}

		if err := verifyResultWithDelay(func() (bool, error) {
			if tcs, end := channelz.GetTopChannels(c.start); len(tcs) != 0 || !end {
				return false, fmt.Errorf("GetTopChannels(0) = %+v (len of which: %d), end: %+v, want len(GetTopChannels(0)) = 0, end: true", tcs, len(tcs), end)
			}
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}
		te.tearDown()
	}
}

func TestCZNestedChannelRegistrationAndDeletion(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	// avoid calling API to set balancer type, which will void service config's change of balancer.
	e.balancer = ""
	te := newTest(t, e)
	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()
	resolvedAddrs := []resolver.Address{{Addr: "127.0.0.1:0", Type: resolver.GRPCLB, ServerName: "grpclb.server"}}
	r.InitialAddrs(resolvedAddrs)
	te.resolverScheme = r.Scheme()
	te.clientConn()
	defer te.tearDown()

	if err := verifyResultWithDelay(func() (bool, error) {
		tcs, _ := channelz.GetTopChannels(0)
		if len(tcs) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tcs))
		}
		if len(tcs[0].NestedChans) != 1 {
			return false, fmt.Errorf("There should be one nested channel from grpclb, not %d", len(tcs[0].NestedChans))
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	r.NewServiceConfig(`{"loadBalancingPolicy": "round_robin"}`)
	r.NewAddress([]resolver.Address{{Addr: "127.0.0.1:0"}})

	// wait for the shutdown of grpclb balancer
	if err := verifyResultWithDelay(func() (bool, error) {
		tcs, _ := channelz.GetTopChannels(0)
		if len(tcs) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tcs))
		}
		if len(tcs[0].NestedChans) != 0 {
			return false, fmt.Errorf("There should be 0 nested channel from grpclb, not %d", len(tcs[0].NestedChans))
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCZClientSubChannelSocketRegistrationAndDeletion(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	num := 3 // number of backends
	te := newTest(t, e)
	var svrAddrs []resolver.Address
	te.startServers(&testServer{security: e.security}, num)
	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()
	for _, a := range te.srvAddrs {
		svrAddrs = append(svrAddrs, resolver.Address{Addr: a})
	}
	r.InitialAddrs(svrAddrs)
	te.resolverScheme = r.Scheme()
	te.clientConn()
	defer te.tearDown()
	// Here, we just wait for all sockets to be up. In the future, if we implement
	// IDLE, we may need to make several rpc calls to create the sockets.
	if err := verifyResultWithDelay(func() (bool, error) {
		tcs, _ := channelz.GetTopChannels(0)
		if len(tcs) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tcs))
		}
		if len(tcs[0].SubChans) != num {
			return false, fmt.Errorf("There should be %d subchannel not %d", num, len(tcs[0].SubChans))
		}
		count := 0
		for k := range tcs[0].SubChans {
			sc := channelz.GetSubChannel(k)
			if sc == nil {
				return false, fmt.Errorf("got <nil> subchannel")
			}
			count += len(sc.Sockets)
		}
		if count != num {
			return false, fmt.Errorf("There should be %d sockets not %d", num, count)
		}

		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	r.NewAddress(svrAddrs[:len(svrAddrs)-1])

	if err := verifyResultWithDelay(func() (bool, error) {
		tcs, _ := channelz.GetTopChannels(0)
		if len(tcs) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tcs))
		}
		if len(tcs[0].SubChans) != num-1 {
			return false, fmt.Errorf("There should be %d subchannel not %d", num-1, len(tcs[0].SubChans))
		}
		count := 0
		for k := range tcs[0].SubChans {
			sc := channelz.GetSubChannel(k)
			if sc == nil {
				return false, fmt.Errorf("got <nil> subchannel")
			}
			count += len(sc.Sockets)
		}
		if count != num-1 {
			return false, fmt.Errorf("There should be %d sockets not %d", num-1, count)
		}

		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCZServerSocketRegistrationAndDeletion(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	num := 3 // number of clients
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	var ccs []*grpc.ClientConn
	for i := 0; i < num; i++ {
		cc := te.clientConn()
		te.cc = nil
		ccs = append(ccs, cc)
	}
	defer func() {
		for _, c := range ccs[:len(ccs)-1] {
			c.Close()
		}
	}()
	var svrID int64
	if err := verifyResultWithDelay(func() (bool, error) {
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should only be one server, not %d", len(ss))
		}
		if len(ss[0].ListenSockets) != 1 {
			return false, fmt.Errorf("There should only be one server listen socket, not %d", len(ss[0].ListenSockets))
		}
		ns, _ := channelz.GetServerSockets(ss[0].ID, 0)
		if len(ns) != num {
			return false, fmt.Errorf("There should be %d normal sockets not %d", num, len(ns))
		}
		svrID = ss[0].ID
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	ccs[len(ccs)-1].Close()

	if err := verifyResultWithDelay(func() (bool, error) {
		ns, _ := channelz.GetServerSockets(svrID, 0)
		if len(ns) != num-1 {
			return false, fmt.Errorf("There should be %d normal sockets not %d", num-1, len(ns))
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCZServerListenSocketDeletion(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	s := grpc.NewServer()
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	go s.Serve(lis)
	if err := verifyResultWithDelay(func() (bool, error) {
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should only be one server, not %d", len(ss))
		}
		if len(ss[0].ListenSockets) != 1 {
			return false, fmt.Errorf("There should only be one server listen socket, not %d", len(ss[0].ListenSockets))
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	lis.Close()
	if err := verifyResultWithDelay(func() (bool, error) {
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should be 1 server, not %d", len(ss))
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	s.Stop()
}

type dummyChannel struct{}

func (d *dummyChannel) ChannelzMetric() *channelz.ChannelInternalMetric {
	return &channelz.ChannelInternalMetric{}
}

type dummySocket struct{}

func (d *dummySocket) ChannelzMetric() *channelz.SocketInternalMetric {
	return &channelz.SocketInternalMetric{}
}

func TestCZRecusivelyDeletionOfEntry(t *testing.T) {
	//           +--+TopChan+---+
	//           |              |
	//           v              v
	//    +-+SubChan1+--+   SubChan2
	//    |             |
	//    v             v
	// Socket1       Socket2
	channelz.NewChannelzStorage()
	topChanID := channelz.RegisterChannel(&dummyChannel{}, 0, "")
	subChanID1 := channelz.RegisterSubChannel(&dummyChannel{}, topChanID, "")
	subChanID2 := channelz.RegisterSubChannel(&dummyChannel{}, topChanID, "")
	sktID1 := channelz.RegisterNormalSocket(&dummySocket{}, subChanID1, "")
	sktID2 := channelz.RegisterNormalSocket(&dummySocket{}, subChanID1, "")

	tcs, _ := channelz.GetTopChannels(0)
	if tcs == nil || len(tcs) != 1 {
		t.Fatalf("There should be one TopChannel entry")
	}
	if len(tcs[0].SubChans) != 2 {
		t.Fatalf("There should be two SubChannel entries")
	}
	sc := channelz.GetSubChannel(subChanID1)
	if sc == nil || len(sc.Sockets) != 2 {
		t.Fatalf("There should be two Socket entries")
	}

	channelz.RemoveEntry(topChanID)
	tcs, _ = channelz.GetTopChannels(0)
	if tcs == nil || len(tcs) != 1 {
		t.Fatalf("There should be one TopChannel entry")
	}

	channelz.RemoveEntry(subChanID1)
	channelz.RemoveEntry(subChanID2)
	tcs, _ = channelz.GetTopChannels(0)
	if tcs == nil || len(tcs) != 1 {
		t.Fatalf("There should be one TopChannel entry")
	}
	if len(tcs[0].SubChans) != 1 {
		t.Fatalf("There should be one SubChannel entry")
	}

	channelz.RemoveEntry(sktID1)
	channelz.RemoveEntry(sktID2)
	tcs, _ = channelz.GetTopChannels(0)
	if tcs != nil {
		t.Fatalf("There should be no TopChannel entry")
	}
}

func TestCZChannelMetrics(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	num := 3 // number of backends
	te := newTest(t, e)
	te.maxClientSendMsgSize = newInt(8)
	var svrAddrs []resolver.Address
	te.startServers(&testServer{security: e.security}, num)
	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()
	for _, a := range te.srvAddrs {
		svrAddrs = append(svrAddrs, resolver.Address{Addr: a})
	}
	r.InitialAddrs(svrAddrs)
	te.resolverScheme = r.Scheme()
	cc := te.clientConn()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}

	const smallSize = 1
	const largeSize = 8

	largePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, largeSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(smallSize),
		Payload:      largePayload,
	}

	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	defer stream.CloseSend()
	// Here, we just wait for all sockets to be up. In the future, if we implement
	// IDLE, we may need to make several rpc calls to create the sockets.
	if err := verifyResultWithDelay(func() (bool, error) {
		tcs, _ := channelz.GetTopChannels(0)
		if len(tcs) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tcs))
		}
		if len(tcs[0].SubChans) != num {
			return false, fmt.Errorf("There should be %d subchannel not %d", num, len(tcs[0].SubChans))
		}
		var cst, csu, cf int64
		for k := range tcs[0].SubChans {
			sc := channelz.GetSubChannel(k)
			if sc == nil {
				return false, fmt.Errorf("got <nil> subchannel")
			}
			cst += sc.ChannelData.CallsStarted
			csu += sc.ChannelData.CallsSucceeded
			cf += sc.ChannelData.CallsFailed
		}
		if cst != 3 {
			return false, fmt.Errorf("There should be 3 CallsStarted not %d", cst)
		}
		if csu != 1 {
			return false, fmt.Errorf("There should be 1 CallsSucceeded not %d", csu)
		}
		if cf != 1 {
			return false, fmt.Errorf("There should be 1 CallsFailed not %d", cf)
		}
		if tcs[0].ChannelData.CallsStarted != 3 {
			return false, fmt.Errorf("There should be 3 CallsStarted not %d", tcs[0].ChannelData.CallsStarted)
		}
		if tcs[0].ChannelData.CallsSucceeded != 1 {
			return false, fmt.Errorf("There should be 1 CallsSucceeded not %d", tcs[0].ChannelData.CallsSucceeded)
		}
		if tcs[0].ChannelData.CallsFailed != 1 {
			return false, fmt.Errorf("There should be 1 CallsFailed not %d", tcs[0].ChannelData.CallsFailed)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCZServerMetrics(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	te := newTest(t, e)
	te.maxServerReceiveMsgSize = newInt(8)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}

	const smallSize = 1
	const largeSize = 8

	largePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, largeSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(smallSize),
		Payload:      largePayload,
	}
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	defer stream.CloseSend()

	if err := verifyResultWithDelay(func() (bool, error) {
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should only be one server, not %d", len(ss))
		}
		if ss[0].ServerData.CallsStarted != 3 {
			return false, fmt.Errorf("There should be 3 CallsStarted not %d", ss[0].ServerData.CallsStarted)
		}
		if ss[0].ServerData.CallsSucceeded != 1 {
			return false, fmt.Errorf("There should be 1 CallsSucceeded not %d", ss[0].ServerData.CallsSucceeded)
		}
		if ss[0].ServerData.CallsFailed != 1 {
			return false, fmt.Errorf("There should be 1 CallsFailed not %d", ss[0].ServerData.CallsFailed)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

type testServiceClientWrapper struct {
	testpb.TestServiceClient
	mu             sync.RWMutex
	streamsCreated int
}

func (t *testServiceClientWrapper) getCurrentStreamID() uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return uint32(2*t.streamsCreated - 1)
}

func (t *testServiceClientWrapper) EmptyCall(ctx context.Context, in *testpb.Empty, opts ...grpc.CallOption) (*testpb.Empty, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamsCreated++
	return t.TestServiceClient.EmptyCall(ctx, in, opts...)
}

func (t *testServiceClientWrapper) UnaryCall(ctx context.Context, in *testpb.SimpleRequest, opts ...grpc.CallOption) (*testpb.SimpleResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamsCreated++
	return t.TestServiceClient.UnaryCall(ctx, in, opts...)
}

func (t *testServiceClientWrapper) StreamingOutputCall(ctx context.Context, in *testpb.StreamingOutputCallRequest, opts ...grpc.CallOption) (testpb.TestService_StreamingOutputCallClient, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamsCreated++
	return t.TestServiceClient.StreamingOutputCall(ctx, in, opts...)
}

func (t *testServiceClientWrapper) StreamingInputCall(ctx context.Context, opts ...grpc.CallOption) (testpb.TestService_StreamingInputCallClient, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamsCreated++
	return t.TestServiceClient.StreamingInputCall(ctx, opts...)
}

func (t *testServiceClientWrapper) FullDuplexCall(ctx context.Context, opts ...grpc.CallOption) (testpb.TestService_FullDuplexCallClient, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamsCreated++
	return t.TestServiceClient.FullDuplexCall(ctx, opts...)
}

func (t *testServiceClientWrapper) HalfDuplexCall(ctx context.Context, opts ...grpc.CallOption) (testpb.TestService_HalfDuplexCallClient, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamsCreated++
	return t.TestServiceClient.HalfDuplexCall(ctx, opts...)
}

func doSuccessfulUnaryCall(tc testpb.TestServiceClient, t *testing.T) {
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}
}

func doStreamingInputCallWithLargePayload(tc testpb.TestServiceClient, t *testing.T) {
	s, err := tc.StreamingInputCall(context.Background())
	if err != nil {
		t.Fatalf("TestService/StreamingInputCall(_) = _, %v, want <nil>", err)
	}
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 10000)
	if err != nil {
		t.Fatal(err)
	}
	s.Send(&testpb.StreamingInputCallRequest{Payload: payload})
}

func doServerSideFailedUnaryCall(tc testpb.TestServiceClient, t *testing.T) {
	const smallSize = 1
	const largeSize = 2000

	largePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, largeSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(smallSize),
		Payload:      largePayload,
	}
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}
}

func doClientSideInitiatedFailedStream(tc testpb.TestServiceClient, t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want <nil>", err)
	}

	const smallSize = 1
	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}

	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: []*testpb.ResponseParameters{
			{Size: smallSize},
		},
		Payload: smallPayload,
	}

	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
	}
	// By canceling the call, the client will send rst_stream to end the call, and
	// the stream will failed as a result.
	cancel()
}

// This func is to be used to test client side counting of failed streams.
func doServerSideInitiatedFailedStreamWithRSTStream(tc testpb.TestServiceClient, t *testing.T, l *listenerWrapper) {
	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want <nil>", err)
	}

	const smallSize = 1
	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}

	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: []*testpb.ResponseParameters{
			{Size: smallSize},
		},
		Payload: smallPayload,
	}

	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
	}

	rcw := l.getLastConn()

	if rcw != nil {
		rcw.writeRSTStream(tc.(*testServiceClientWrapper).getCurrentStreamID(), http2.ErrCodeCancel)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatalf("%v.Recv() = %v, want <non-nil>", stream, err)
	}
}

// this func is to be used to test client side counting of failed streams.
func doServerSideInitiatedFailedStreamWithGoAway(tc testpb.TestServiceClient, t *testing.T, l *listenerWrapper) {
	// This call is just to keep the transport from shutting down (socket will be deleted
	// in this case, and we will not be able to get metrics).
	s, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want <nil>", err)
	}
	if err := s.Send(&testpb.StreamingOutputCallRequest{ResponseParameters: []*testpb.ResponseParameters{
		{
			Size: 1,
		},
	}}); err != nil {
		t.Fatalf("s.Send() failed with error: %v", err)
	}
	if _, err := s.Recv(); err != nil {
		t.Fatalf("s.Recv() failed with error: %v", err)
	}

	s, err = tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want <nil>", err)
	}
	if err := s.Send(&testpb.StreamingOutputCallRequest{ResponseParameters: []*testpb.ResponseParameters{
		{
			Size: 1,
		},
	}}); err != nil {
		t.Fatalf("s.Send() failed with error: %v", err)
	}
	if _, err := s.Recv(); err != nil {
		t.Fatalf("s.Recv() failed with error: %v", err)
	}

	rcw := l.getLastConn()
	if rcw != nil {
		rcw.writeGoAway(tc.(*testServiceClientWrapper).getCurrentStreamID()-2, http2.ErrCodeCancel, []byte{})
	}
	if _, err := s.Recv(); err == nil {
		t.Fatalf("%v.Recv() = %v, want <non-nil>", s, err)
	}
}

// this func is to be used to test client side counting of failed streams.
func doServerSideInitiatedFailedStreamWithClientBreakFlowControl(tc testpb.TestServiceClient, t *testing.T, dw *dialerWrapper) {
	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want <nil>", err)
	}
	// sleep here to make sure header frame being sent before the data frame we write directly below.
	time.Sleep(10 * time.Millisecond)
	payload := make([]byte, 65537, 65537)
	dw.getRawConnWrapper().writeRawFrame(http2.FrameData, 0, tc.(*testServiceClientWrapper).getCurrentStreamID(), payload)
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = %v, want error code: %v", stream, err, codes.ResourceExhausted)
	}
}

func doIdleCallToInvokeKeepAlive(tc testpb.TestServiceClient, t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want <nil>", err)
	}
	// 2500ms allow for 2 keepalives (1000ms per round trip)
	time.Sleep(2500 * time.Millisecond)
	cancel()
}

func TestCZClientSocketMetricsStreamsAndMessagesCount(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	te := newTest(t, e)
	te.maxServerReceiveMsgSize = newInt(20)
	te.maxClientReceiveMsgSize = newInt(20)
	rcw := te.startServerWithConnControl(&testServer{security: e.security})
	defer te.tearDown()
	cc := te.clientConn()
	tc := &testServiceClientWrapper{TestServiceClient: testpb.NewTestServiceClient(cc)}

	doSuccessfulUnaryCall(tc, t)
	var scID, skID int64
	if err := verifyResultWithDelay(func() (bool, error) {
		tchan, _ := channelz.GetTopChannels(0)
		if len(tchan) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tchan))
		}
		if len(tchan[0].SubChans) != 1 {
			return false, fmt.Errorf("There should only be one subchannel under top channel %d, not %d", tchan[0].ID, len(tchan[0].SubChans))
		}

		for scID = range tchan[0].SubChans {
			break
		}
		sc := channelz.GetSubChannel(scID)
		if sc == nil {
			return false, fmt.Errorf("There should only be one socket under subchannel %d, not 0", scID)
		}
		if len(sc.Sockets) != 1 {
			return false, fmt.Errorf("There should only be one socket under subchannel %d, not %d", sc.ID, len(sc.Sockets))
		}
		for skID = range sc.Sockets {
			break
		}
		skt := channelz.GetSocket(skID)
		sktData := skt.SocketData
		if sktData.StreamsStarted != 1 || sktData.StreamsSucceeded != 1 || sktData.MessagesSent != 1 || sktData.MessagesReceived != 1 {
			return false, fmt.Errorf("channelz.GetSocket(%d), want (StreamsStarted, StreamsSucceeded, MessagesSent, MessagesReceived) = (1, 1, 1, 1), got (%d, %d, %d, %d)", skt.ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.MessagesSent, sktData.MessagesReceived)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	doServerSideFailedUnaryCall(tc, t)
	if err := verifyResultWithDelay(func() (bool, error) {
		skt := channelz.GetSocket(skID)
		sktData := skt.SocketData
		if sktData.StreamsStarted != 2 || sktData.StreamsSucceeded != 2 || sktData.MessagesSent != 2 || sktData.MessagesReceived != 1 {
			return false, fmt.Errorf("channelz.GetSocket(%d), want (StreamsStarted, StreamsSucceeded, MessagesSent, MessagesReceived) = (2, 2, 2, 1), got (%d, %d, %d, %d)", skt.ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.MessagesSent, sktData.MessagesReceived)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	doClientSideInitiatedFailedStream(tc, t)
	if err := verifyResultWithDelay(func() (bool, error) {
		skt := channelz.GetSocket(skID)
		sktData := skt.SocketData
		if sktData.StreamsStarted != 3 || sktData.StreamsSucceeded != 2 || sktData.StreamsFailed != 1 || sktData.MessagesSent != 3 || sktData.MessagesReceived != 2 {
			return false, fmt.Errorf("channelz.GetSocket(%d), want (StreamsStarted, StreamsSucceeded, StreamsFailed, MessagesSent, MessagesReceived) = (3, 2, 1, 3, 2), got (%d, %d, %d, %d, %d)", skt.ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.StreamsFailed, sktData.MessagesSent, sktData.MessagesReceived)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	doServerSideInitiatedFailedStreamWithRSTStream(tc, t, rcw)
	if err := verifyResultWithDelay(func() (bool, error) {
		skt := channelz.GetSocket(skID)
		sktData := skt.SocketData
		if sktData.StreamsStarted != 4 || sktData.StreamsSucceeded != 2 || sktData.StreamsFailed != 2 || sktData.MessagesSent != 4 || sktData.MessagesReceived != 3 {
			return false, fmt.Errorf("channelz.GetSocket(%d), want (StreamsStarted, StreamsSucceeded, StreamsFailed, MessagesSent, MessagesReceived) = (4, 2, 2, 4, 3), got (%d, %d, %d, %d, %d)", skt.ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.StreamsFailed, sktData.MessagesSent, sktData.MessagesReceived)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	doServerSideInitiatedFailedStreamWithGoAway(tc, t, rcw)
	if err := verifyResultWithDelay(func() (bool, error) {
		skt := channelz.GetSocket(skID)
		sktData := skt.SocketData
		if sktData.StreamsStarted != 6 || sktData.StreamsSucceeded != 2 || sktData.StreamsFailed != 3 || sktData.MessagesSent != 6 || sktData.MessagesReceived != 5 {
			return false, fmt.Errorf("channelz.GetSocket(%d), want (StreamsStarted, StreamsSucceeded, StreamsFailed, MessagesSent, MessagesReceived) = (6, 2, 3, 6, 5), got (%d, %d, %d, %d, %d)", skt.ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.StreamsFailed, sktData.MessagesSent, sktData.MessagesReceived)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// This test is to complete TestCZClientSocketMetricsStreamsAndMessagesCount and
// TestCZServerSocketMetricsStreamsAndMessagesCount by adding the test case of
// server sending RST_STREAM to client due to client side flow control violation.
// It is separated from other cases due to setup incompatibly, i.e. max receive
// size violation will mask flow control violation.
func TestCZClientAndServerSocketMetricsStreamsCountFlowControlRSTStream(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	te := newTest(t, e)
	te.serverInitialWindowSize = 65536
	// Avoid overflowing connection level flow control window, which will lead to
	// transport being closed.
	te.serverInitialConnWindowSize = 65536 * 2
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	cc, dw := te.clientConnWithConnControl()
	tc := &testServiceClientWrapper{TestServiceClient: testpb.NewTestServiceClient(cc)}

	doServerSideInitiatedFailedStreamWithClientBreakFlowControl(tc, t, dw)
	if err := verifyResultWithDelay(func() (bool, error) {
		tchan, _ := channelz.GetTopChannels(0)
		if len(tchan) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tchan))
		}
		if len(tchan[0].SubChans) != 1 {
			return false, fmt.Errorf("There should only be one subchannel under top channel %d, not %d", tchan[0].ID, len(tchan[0].SubChans))
		}
		var id int64
		for id = range tchan[0].SubChans {
			break
		}
		sc := channelz.GetSubChannel(id)
		if sc == nil {
			return false, fmt.Errorf("There should only be one socket under subchannel %d, not 0", id)
		}
		if len(sc.Sockets) != 1 {
			return false, fmt.Errorf("There should only be one socket under subchannel %d, not %d", sc.ID, len(sc.Sockets))
		}
		for id = range sc.Sockets {
			break
		}
		skt := channelz.GetSocket(id)
		sktData := skt.SocketData
		if sktData.StreamsStarted != 1 || sktData.StreamsSucceeded != 0 || sktData.StreamsFailed != 1 {
			return false, fmt.Errorf("channelz.GetSocket(%d), want (StreamsStarted, StreamsSucceeded, StreamsFailed) = (1, 0, 1), got (%d, %d, %d)", skt.ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.StreamsFailed)
		}
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should only be one server, not %d", len(ss))
		}

		ns, _ := channelz.GetServerSockets(ss[0].ID, 0)
		if len(ns) != 1 {
			return false, fmt.Errorf("There should be one server normal socket, not %d", len(ns))
		}
		sktData = ns[0].SocketData
		if sktData.StreamsStarted != 1 || sktData.StreamsSucceeded != 0 || sktData.StreamsFailed != 1 {
			return false, fmt.Errorf("Server socket metric with ID %d, want (StreamsStarted, StreamsSucceeded, StreamsFailed) = (1, 0, 1), got (%d, %d, %d)", ns[0].ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.StreamsFailed)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCZClientAndServerSocketMetricsFlowControl(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	te := newTest(t, e)
	// disable BDP
	te.serverInitialWindowSize = 65536
	te.serverInitialConnWindowSize = 65536
	te.clientInitialWindowSize = 65536
	te.clientInitialConnWindowSize = 65536
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	for i := 0; i < 10; i++ {
		doSuccessfulUnaryCall(tc, t)
	}

	var cliSktID, svrSktID int64
	if err := verifyResultWithDelay(func() (bool, error) {
		tchan, _ := channelz.GetTopChannels(0)
		if len(tchan) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tchan))
		}
		if len(tchan[0].SubChans) != 1 {
			return false, fmt.Errorf("There should only be one subchannel under top channel %d, not %d", tchan[0].ID, len(tchan[0].SubChans))
		}
		var id int64
		for id = range tchan[0].SubChans {
			break
		}
		sc := channelz.GetSubChannel(id)
		if sc == nil {
			return false, fmt.Errorf("There should only be one socket under subchannel %d, not 0", id)
		}
		if len(sc.Sockets) != 1 {
			return false, fmt.Errorf("There should only be one socket under subchannel %d, not %d", sc.ID, len(sc.Sockets))
		}
		for id = range sc.Sockets {
			break
		}
		skt := channelz.GetSocket(id)
		sktData := skt.SocketData
		// 65536 - 5 (Length-Prefixed-Message size) * 10 = 65486
		if sktData.LocalFlowControlWindow != 65486 || sktData.RemoteFlowControlWindow != 65486 {
			return false, fmt.Errorf("Client: (LocalFlowControlWindow, RemoteFlowControlWindow) size should be (65536, 65486), not (%d, %d)", sktData.LocalFlowControlWindow, sktData.RemoteFlowControlWindow)
		}
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should only be one server, not %d", len(ss))
		}
		ns, _ := channelz.GetServerSockets(ss[0].ID, 0)
		sktData = ns[0].SocketData
		if sktData.LocalFlowControlWindow != 65486 || sktData.RemoteFlowControlWindow != 65486 {
			return false, fmt.Errorf("Server: (LocalFlowControlWindow, RemoteFlowControlWindow) size should be (65536, 65486), not (%d, %d)", sktData.LocalFlowControlWindow, sktData.RemoteFlowControlWindow)
		}
		cliSktID, svrSktID = id, ss[0].ID
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	doStreamingInputCallWithLargePayload(tc, t)

	if err := verifyResultWithDelay(func() (bool, error) {
		skt := channelz.GetSocket(cliSktID)
		sktData := skt.SocketData
		// Local: 65536 - 5 (Length-Prefixed-Message size) * 10 = 65486
		// Remote: 65536 - 5 (Length-Prefixed-Message size) * 10 - 10011 = 55475
		if sktData.LocalFlowControlWindow != 65486 || sktData.RemoteFlowControlWindow != 55475 {
			return false, fmt.Errorf("Client: (LocalFlowControlWindow, RemoteFlowControlWindow) size should be (65486, 55475), not (%d, %d)", sktData.LocalFlowControlWindow, sktData.RemoteFlowControlWindow)
		}
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should only be one server, not %d", len(ss))
		}
		ns, _ := channelz.GetServerSockets(svrSktID, 0)
		sktData = ns[0].SocketData
		if sktData.LocalFlowControlWindow != 55475 || sktData.RemoteFlowControlWindow != 65486 {
			return false, fmt.Errorf("Server: (LocalFlowControlWindow, RemoteFlowControlWindow) size should be (55475, 65486), not (%d, %d)", sktData.LocalFlowControlWindow, sktData.RemoteFlowControlWindow)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	// triggers transport flow control window update on server side, since unacked
	// bytes should be larger than limit now. i.e. 50 + 20022 > 65536/4.
	doStreamingInputCallWithLargePayload(tc, t)
	if err := verifyResultWithDelay(func() (bool, error) {
		skt := channelz.GetSocket(cliSktID)
		sktData := skt.SocketData
		// Local: 65536 - 5 (Length-Prefixed-Message size) * 10 = 65486
		// Remote: 65536
		if sktData.LocalFlowControlWindow != 65486 || sktData.RemoteFlowControlWindow != 65536 {
			return false, fmt.Errorf("Client: (LocalFlowControlWindow, RemoteFlowControlWindow) size should be (65486, 65536), not (%d, %d)", sktData.LocalFlowControlWindow, sktData.RemoteFlowControlWindow)
		}
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should only be one server, not %d", len(ss))
		}
		ns, _ := channelz.GetServerSockets(svrSktID, 0)
		sktData = ns[0].SocketData
		if sktData.LocalFlowControlWindow != 65536 || sktData.RemoteFlowControlWindow != 65486 {
			return false, fmt.Errorf("Server: (LocalFlowControlWindow, RemoteFlowControlWindow) size should be (65536, 65486), not (%d, %d)", sktData.LocalFlowControlWindow, sktData.RemoteFlowControlWindow)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCZClientSocketMetricsKeepAlive(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	te := newTest(t, e)
	te.cliKeepAlive = &keepalive.ClientParameters{Time: 500 * time.Millisecond, Timeout: 500 * time.Millisecond}
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	doIdleCallToInvokeKeepAlive(tc, t)

	if err := verifyResultWithDelay(func() (bool, error) {
		tchan, _ := channelz.GetTopChannels(0)
		if len(tchan) != 1 {
			return false, fmt.Errorf("There should only be one top channel, not %d", len(tchan))
		}
		if len(tchan[0].SubChans) != 1 {
			return false, fmt.Errorf("There should only be one subchannel under top channel %d, not %d", tchan[0].ID, len(tchan[0].SubChans))
		}
		var id int64
		for id = range tchan[0].SubChans {
			break
		}
		sc := channelz.GetSubChannel(id)
		if sc == nil {
			return false, fmt.Errorf("There should only be one socket under subchannel %d, not 0", id)
		}
		if len(sc.Sockets) != 1 {
			return false, fmt.Errorf("There should only be one socket under subchannel %d, not %d", sc.ID, len(sc.Sockets))
		}
		for id = range sc.Sockets {
			break
		}
		skt := channelz.GetSocket(id)
		if skt.SocketData.KeepAlivesSent != 2 { // doIdleCallToInvokeKeepAlive func is set up to send 2 KeepAlives.
			return false, fmt.Errorf("There should be 2 KeepAlives sent, not %d", skt.SocketData.KeepAlivesSent)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCZServerSocketMetricsStreamsAndMessagesCount(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	te := newTest(t, e)
	te.maxServerReceiveMsgSize = newInt(20)
	te.maxClientReceiveMsgSize = newInt(20)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	cc, _ := te.clientConnWithConnControl()
	tc := &testServiceClientWrapper{TestServiceClient: testpb.NewTestServiceClient(cc)}

	var svrID int64
	if err := verifyResultWithDelay(func() (bool, error) {
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should only be one server, not %d", len(ss))
		}
		svrID = ss[0].ID
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	doSuccessfulUnaryCall(tc, t)
	if err := verifyResultWithDelay(func() (bool, error) {
		ns, _ := channelz.GetServerSockets(svrID, 0)
		sktData := ns[0].SocketData
		if sktData.StreamsStarted != 1 || sktData.StreamsSucceeded != 1 || sktData.StreamsFailed != 0 || sktData.MessagesSent != 1 || sktData.MessagesReceived != 1 {
			return false, fmt.Errorf("Server socket metric with ID %d, want (StreamsStarted, StreamsSucceeded, MessagesSent, MessagesReceived) = (1, 1, 1, 1), got (%d, %d, %d, %d, %d)", ns[0].ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.StreamsFailed, sktData.MessagesSent, sktData.MessagesReceived)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	doServerSideFailedUnaryCall(tc, t)
	if err := verifyResultWithDelay(func() (bool, error) {
		ns, _ := channelz.GetServerSockets(svrID, 0)
		sktData := ns[0].SocketData
		if sktData.StreamsStarted != 2 || sktData.StreamsSucceeded != 2 || sktData.StreamsFailed != 0 || sktData.MessagesSent != 1 || sktData.MessagesReceived != 1 {
			return false, fmt.Errorf("Server socket metric with ID %d, want (StreamsStarted, StreamsSucceeded, StreamsFailed, MessagesSent, MessagesReceived) = (2, 2, 0, 1, 1), got (%d, %d, %d, %d, %d)", ns[0].ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.StreamsFailed, sktData.MessagesSent, sktData.MessagesReceived)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	doClientSideInitiatedFailedStream(tc, t)
	if err := verifyResultWithDelay(func() (bool, error) {
		ns, _ := channelz.GetServerSockets(svrID, 0)
		sktData := ns[0].SocketData
		if sktData.StreamsStarted != 3 || sktData.StreamsSucceeded != 2 || sktData.StreamsFailed != 1 || sktData.MessagesSent != 2 || sktData.MessagesReceived != 2 {
			return false, fmt.Errorf("Server socket metric with ID %d, want (StreamsStarted, StreamsSucceeded, StreamsFailed, MessagesSent, MessagesReceived) = (3, 2, 1, 2, 2), got (%d, %d, %d, %d, %d)", ns[0].ID, sktData.StreamsStarted, sktData.StreamsSucceeded, sktData.StreamsFailed, sktData.MessagesSent, sktData.MessagesReceived)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCZServerSocketMetricsKeepAlive(t *testing.T) {
	defer leakcheck.Check(t)
	channelz.NewChannelzStorage()
	e := tcpClearRREnv
	te := newTest(t, e)
	te.svrKeepAlive = &keepalive.ServerParameters{Time: 500 * time.Millisecond, Timeout: 500 * time.Millisecond}
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	doIdleCallToInvokeKeepAlive(tc, t)

	if err := verifyResultWithDelay(func() (bool, error) {
		ss, _ := channelz.GetServers(0)
		if len(ss) != 1 {
			return false, fmt.Errorf("There should be one server, not %d", len(ss))
		}
		ns, _ := channelz.GetServerSockets(ss[0].ID, 0)
		if len(ns) != 1 {
			return false, fmt.Errorf("There should be one server normal socket, not %d", len(ns))
		}
		if ns[0].SocketData.KeepAlivesSent != 2 { // doIdleCallToInvokeKeepAlive func is set up to send 2 KeepAlives.
			return false, fmt.Errorf("There should be 2 KeepAlives sent, not %d", ns[0].SocketData.KeepAlivesSent)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}
