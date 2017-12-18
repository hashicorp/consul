/*
 *
 * Copyright 2016 gRPC authors.
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

//go:generate protoc --go_out=plugins=:$GOPATH grpc_lb_v1/messages/messages.proto
//go:generate protoc --go_out=plugins=grpc:$GOPATH grpc_lb_v1/service/service.proto

// Package grpclb_test is currently used only for grpclb testing.
package grpclb_test

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	lbmpb "google.golang.org/grpc/grpclb/grpc_lb_v1/messages"
	lbspb "google.golang.org/grpc/grpclb/grpc_lb_v1/service"
	_ "google.golang.org/grpc/grpclog/glogger"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/naming"
	"google.golang.org/grpc/status"
	testpb "google.golang.org/grpc/test/grpc_testing"
	"google.golang.org/grpc/test/leakcheck"
)

var (
	lbsn    = "bar.com"
	besn    = "foo.com"
	lbToken = "iamatoken"

	// Resolver replaces localhost with fakeName in Next().
	// Dialer replaces fakeName with localhost when dialing.
	// This will test that custom dialer is passed from Dial to grpclb.
	fakeName = "fake.Name"
)

type testWatcher struct {
	// the channel to receives name resolution updates
	update chan *naming.Update
	// the side channel to get to know how many updates in a batch
	side chan int
	// the channel to notifiy update injector that the update reading is done
	readDone chan int
}

func (w *testWatcher) Next() (updates []*naming.Update, err error) {
	n, ok := <-w.side
	if !ok {
		return nil, fmt.Errorf("w.side is closed")
	}
	for i := 0; i < n; i++ {
		u, ok := <-w.update
		if !ok {
			break
		}
		if u != nil {
			// Resolver replaces localhost with fakeName in Next().
			// Custom dialer will replace fakeName with localhost when dialing.
			u.Addr = strings.Replace(u.Addr, "localhost", fakeName, 1)
			updates = append(updates, u)
		}
	}
	w.readDone <- 0
	return
}

func (w *testWatcher) Close() {
	close(w.side)
}

// Inject naming resolution updates to the testWatcher.
func (w *testWatcher) inject(updates []*naming.Update) {
	w.side <- len(updates)
	for _, u := range updates {
		w.update <- u
	}
	<-w.readDone
}

type testNameResolver struct {
	w     *testWatcher
	addrs []string
}

func (r *testNameResolver) Resolve(target string) (naming.Watcher, error) {
	r.w = &testWatcher{
		update:   make(chan *naming.Update, len(r.addrs)),
		side:     make(chan int, 1),
		readDone: make(chan int),
	}
	r.w.side <- len(r.addrs)
	for _, addr := range r.addrs {
		r.w.update <- &naming.Update{
			Op:   naming.Add,
			Addr: addr,
			Metadata: &naming.AddrMetadataGRPCLB{
				AddrType:   naming.GRPCLB,
				ServerName: lbsn,
			},
		}
	}
	go func() {
		<-r.w.readDone
	}()
	return r.w, nil
}

func (r *testNameResolver) inject(updates []*naming.Update) {
	if r.w != nil {
		r.w.inject(updates)
	}
}

type serverNameCheckCreds struct {
	mu       sync.Mutex
	sn       string
	expected string
}

func (c *serverNameCheckCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if _, err := io.WriteString(rawConn, c.sn); err != nil {
		fmt.Printf("Failed to write the server name %s to the client %v", c.sn, err)
		return nil, nil, err
	}
	return rawConn, nil, nil
}
func (c *serverNameCheckCreds) ClientHandshake(ctx context.Context, addr string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := make([]byte, len(c.expected))
	errCh := make(chan error, 1)
	go func() {
		_, err := rawConn.Read(b)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		if err != nil {
			fmt.Printf("Failed to read the server name from the server %v", err)
			return nil, nil, err
		}
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	if c.expected != string(b) {
		fmt.Printf("Read the server name %s want %s", string(b), c.expected)
		return nil, nil, errors.New("received unexpected server name")
	}
	return rawConn, nil, nil
}
func (c *serverNameCheckCreds) Info() credentials.ProtocolInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return credentials.ProtocolInfo{}
}
func (c *serverNameCheckCreds) Clone() credentials.TransportCredentials {
	c.mu.Lock()
	defer c.mu.Unlock()
	return &serverNameCheckCreds{
		expected: c.expected,
	}
}
func (c *serverNameCheckCreds) OverrideServerName(s string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expected = s
	return nil
}

// fakeNameDialer replaces fakeName with localhost when dialing.
// This will test that custom dialer is passed from Dial to grpclb.
func fakeNameDialer(addr string, timeout time.Duration) (net.Conn, error) {
	addr = strings.Replace(addr, fakeName, "localhost", 1)
	return net.DialTimeout("tcp", addr, timeout)
}

type remoteBalancer struct {
	sls       []*lbmpb.ServerList
	intervals []time.Duration
	statsDura time.Duration
	done      chan struct{}
	mu        sync.Mutex
	stats     lbmpb.ClientStats
}

func newRemoteBalancer(sls []*lbmpb.ServerList, intervals []time.Duration) *remoteBalancer {
	return &remoteBalancer{
		sls:       sls,
		intervals: intervals,
		done:      make(chan struct{}),
	}
}

func (b *remoteBalancer) stop() {
	close(b.done)
}

func (b *remoteBalancer) BalanceLoad(stream lbspb.LoadBalancer_BalanceLoadServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	initReq := req.GetInitialRequest()
	if initReq.Name != besn {
		return status.Errorf(codes.InvalidArgument, "invalid service name: %v", initReq.Name)
	}
	resp := &lbmpb.LoadBalanceResponse{
		LoadBalanceResponseType: &lbmpb.LoadBalanceResponse_InitialResponse{
			InitialResponse: &lbmpb.InitialLoadBalanceResponse{
				ClientStatsReportInterval: &lbmpb.Duration{
					Seconds: int64(b.statsDura.Seconds()),
					Nanos:   int32(b.statsDura.Nanoseconds() - int64(b.statsDura.Seconds())*1e9),
				},
			},
		},
	}
	if err := stream.Send(resp); err != nil {
		return err
	}
	go func() {
		for {
			var (
				req *lbmpb.LoadBalanceRequest
				err error
			)
			if req, err = stream.Recv(); err != nil {
				return
			}
			b.mu.Lock()
			b.stats.NumCallsStarted += req.GetClientStats().NumCallsStarted
			b.stats.NumCallsFinished += req.GetClientStats().NumCallsFinished
			b.stats.NumCallsFinishedWithDropForRateLimiting += req.GetClientStats().NumCallsFinishedWithDropForRateLimiting
			b.stats.NumCallsFinishedWithDropForLoadBalancing += req.GetClientStats().NumCallsFinishedWithDropForLoadBalancing
			b.stats.NumCallsFinishedWithClientFailedToSend += req.GetClientStats().NumCallsFinishedWithClientFailedToSend
			b.stats.NumCallsFinishedKnownReceived += req.GetClientStats().NumCallsFinishedKnownReceived
			b.mu.Unlock()
		}
	}()
	for k, v := range b.sls {
		time.Sleep(b.intervals[k])
		resp = &lbmpb.LoadBalanceResponse{
			LoadBalanceResponseType: &lbmpb.LoadBalanceResponse_ServerList{
				ServerList: v,
			},
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	<-b.done
	return nil
}

type testServer struct {
	testpb.TestServiceServer

	addr string
}

const testmdkey = "testmd"

func (s *testServer) EmptyCall(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "failed to receive metadata")
	}
	if md == nil || md["lb-token"][0] != lbToken {
		return nil, status.Errorf(codes.Internal, "received unexpected metadata: %v", md)
	}
	grpc.SetTrailer(ctx, metadata.Pairs(testmdkey, s.addr))
	return &testpb.Empty{}, nil
}

func (s *testServer) FullDuplexCall(stream testpb.TestService_FullDuplexCallServer) error {
	return nil
}

func startBackends(sn string, lis ...net.Listener) (servers []*grpc.Server) {
	for _, l := range lis {
		creds := &serverNameCheckCreds{
			sn: sn,
		}
		s := grpc.NewServer(grpc.Creds(creds))
		testpb.RegisterTestServiceServer(s, &testServer{addr: l.Addr().String()})
		servers = append(servers, s)
		go func(s *grpc.Server, l net.Listener) {
			s.Serve(l)
		}(s, l)
	}
	return
}

func stopBackends(servers []*grpc.Server) {
	for _, s := range servers {
		s.Stop()
	}
}

type testServers struct {
	lbAddr  string
	ls      *remoteBalancer
	lb      *grpc.Server
	beIPs   []net.IP
	bePorts []int
}

func newLoadBalancer(numberOfBackends int) (tss *testServers, cleanup func(), err error) {
	var (
		beListeners []net.Listener
		ls          *remoteBalancer
		lb          *grpc.Server
		beIPs       []net.IP
		bePorts     []int
	)
	for i := 0; i < numberOfBackends; i++ {
		// Start a backend.
		beLis, e := net.Listen("tcp", "localhost:0")
		if e != nil {
			err = fmt.Errorf("Failed to listen %v", err)
			return
		}
		beIPs = append(beIPs, beLis.Addr().(*net.TCPAddr).IP)
		bePorts = append(bePorts, beLis.Addr().(*net.TCPAddr).Port)

		beListeners = append(beListeners, beLis)
	}
	backends := startBackends(besn, beListeners...)

	// Start a load balancer.
	lbLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		err = fmt.Errorf("Failed to create the listener for the load balancer %v", err)
		return
	}
	lbCreds := &serverNameCheckCreds{
		sn: lbsn,
	}
	lb = grpc.NewServer(grpc.Creds(lbCreds))
	if err != nil {
		err = fmt.Errorf("Failed to generate the port number %v", err)
		return
	}
	ls = newRemoteBalancer(nil, nil)
	lbspb.RegisterLoadBalancerServer(lb, ls)
	go func() {
		lb.Serve(lbLis)
	}()

	tss = &testServers{
		lbAddr:  lbLis.Addr().String(),
		ls:      ls,
		lb:      lb,
		beIPs:   beIPs,
		bePorts: bePorts,
	}
	cleanup = func() {
		defer stopBackends(backends)
		defer func() {
			ls.stop()
			lb.Stop()
		}()
	}
	return
}

func TestGRPCLB(t *testing.T) {
	defer leakcheck.Check(t)
	tss, cleanup, err := newLoadBalancer(1)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()

	be := &lbmpb.Server{
		IpAddress:        tss.beIPs[0],
		Port:             int32(tss.bePorts[0]),
		LoadBalanceToken: lbToken,
	}
	var bes []*lbmpb.Server
	bes = append(bes, be)
	sl := &lbmpb.ServerList{
		Servers: bes,
	}
	tss.ls.sls = []*lbmpb.ServerList{sl}
	tss.ls.intervals = []time.Duration{0}
	creds := serverNameCheckCreds{
		expected: besn,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, besn,
		grpc.WithBalancer(grpc.NewGRPCLBBalancer(&testNameResolver{addrs: []string{tss.lbAddr}})),
		grpc.WithBlock(), grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)
	if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
	}
}

func TestDropRequest(t *testing.T) {
	defer leakcheck.Check(t)
	tss, cleanup, err := newLoadBalancer(2)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()
	tss.ls.sls = []*lbmpb.ServerList{{
		Servers: []*lbmpb.Server{{
			IpAddress:            tss.beIPs[0],
			Port:                 int32(tss.bePorts[0]),
			LoadBalanceToken:     lbToken,
			DropForLoadBalancing: true,
		}, {
			IpAddress:            tss.beIPs[1],
			Port:                 int32(tss.bePorts[1]),
			LoadBalanceToken:     lbToken,
			DropForLoadBalancing: false,
		}},
	}}
	tss.ls.intervals = []time.Duration{0}
	creds := serverNameCheckCreds{
		expected: besn,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, besn,
		grpc.WithBalancer(grpc.NewGRPCLBBalancer(&testNameResolver{addrs: []string{tss.lbAddr}})),
		grpc.WithBlock(), grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)
	// Wait until the first connection is up.
	// The first one has Drop set to true, error should contain "drop requests".
	for {
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
			if strings.Contains(err.Error(), "drops requests") {
				break
			}
		}
	}
	// The 1st, non-fail-fast RPC should succeed.  This ensures both server
	// connections are made, because the first one has DropForLoadBalancing set to true.
	if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("%v.SayHello(_, _) = _, %v, want _, <nil>", testC, err)
	}
	for i := 0; i < 3; i++ {
		// Odd fail-fast RPCs should fail, because the 1st backend has DropForLoadBalancing
		// set to true.
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); grpc.Code(err) != codes.Unavailable {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, %s", testC, err, codes.Unavailable)
		}
		// Even fail-fast RPCs should succeed since they choose the
		// non-drop-request backend according to the round robin policy.
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
	}
}

func TestDropRequestFailedNonFailFast(t *testing.T) {
	defer leakcheck.Check(t)
	tss, cleanup, err := newLoadBalancer(1)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()
	be := &lbmpb.Server{
		IpAddress:            tss.beIPs[0],
		Port:                 int32(tss.bePorts[0]),
		LoadBalanceToken:     lbToken,
		DropForLoadBalancing: true,
	}
	var bes []*lbmpb.Server
	bes = append(bes, be)
	sl := &lbmpb.ServerList{
		Servers: bes,
	}
	tss.ls.sls = []*lbmpb.ServerList{sl}
	tss.ls.intervals = []time.Duration{0}
	creds := serverNameCheckCreds{
		expected: besn,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, besn,
		grpc.WithBalancer(grpc.NewGRPCLBBalancer(&testNameResolver{addrs: []string{tss.lbAddr}})),
		grpc.WithBlock(), grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := testC.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false)); grpc.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, %s", testC, err, codes.DeadlineExceeded)
	}
}

// When the balancer in use disconnects, grpclb should connect to the next address from resolved balancer address list.
func TestBalancerDisconnects(t *testing.T) {
	defer leakcheck.Check(t)
	var (
		lbAddrs []string
		lbs     []*grpc.Server
	)
	for i := 0; i < 3; i++ {
		tss, cleanup, err := newLoadBalancer(1)
		if err != nil {
			t.Fatalf("failed to create new load balancer: %v", err)
		}
		defer cleanup()

		be := &lbmpb.Server{
			IpAddress:        tss.beIPs[0],
			Port:             int32(tss.bePorts[0]),
			LoadBalanceToken: lbToken,
		}
		var bes []*lbmpb.Server
		bes = append(bes, be)
		sl := &lbmpb.ServerList{
			Servers: bes,
		}
		tss.ls.sls = []*lbmpb.ServerList{sl}
		tss.ls.intervals = []time.Duration{0}

		lbAddrs = append(lbAddrs, tss.lbAddr)
		lbs = append(lbs, tss.lb)
	}

	creds := serverNameCheckCreds{
		expected: besn,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resolver := &testNameResolver{
		addrs: lbAddrs[:2],
	}
	cc, err := grpc.DialContext(ctx, besn,
		grpc.WithBalancer(grpc.NewGRPCLBBalancer(resolver)),
		grpc.WithBlock(), grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)
	var previousTrailer string
	trailer := metadata.MD{}
	if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.Trailer(&trailer), grpc.FailFast(false)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
	} else {
		previousTrailer = trailer[testmdkey][0]
	}
	// The initial resolver update contains lbs[0] and lbs[1].
	// When lbs[0] is stopped, lbs[1] should be used.
	lbs[0].Stop()
	for {
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.Trailer(&trailer), grpc.FailFast(false)); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		} else if trailer[testmdkey][0] != previousTrailer {
			// A new backend server should receive the request.
			// The trailer contains the backend address, so the trailer should be different from the previous one.
			previousTrailer = trailer[testmdkey][0]
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Inject a update to add lbs[2] to resolved addresses.
	resolver.inject([]*naming.Update{
		{Op: naming.Add,
			Addr: lbAddrs[2],
			Metadata: &naming.AddrMetadataGRPCLB{
				AddrType:   naming.GRPCLB,
				ServerName: lbsn,
			},
		},
	})
	// Stop lbs[1]. Now lbs[0] and lbs[1] are all stopped. lbs[2] should be used.
	lbs[1].Stop()
	for {
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.Trailer(&trailer), grpc.FailFast(false)); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		} else if trailer[testmdkey][0] != previousTrailer {
			// A new backend server should receive the request.
			// The trailer contains the backend address, so the trailer should be different from the previous one.
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type failPreRPCCred struct{}

func (failPreRPCCred) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	if strings.Contains(uri[0], "failtosend") {
		return nil, fmt.Errorf("rpc should fail to send")
	}
	return nil, nil
}

func (failPreRPCCred) RequireTransportSecurity() bool {
	return false
}

func checkStats(stats *lbmpb.ClientStats, expected *lbmpb.ClientStats) error {
	if !proto.Equal(stats, expected) {
		return fmt.Errorf("stats not equal: got %+v, want %+v", stats, expected)
	}
	return nil
}

func runAndGetStats(t *testing.T, dropForLoadBalancing, dropForRateLimiting bool, runRPCs func(*grpc.ClientConn)) lbmpb.ClientStats {
	tss, cleanup, err := newLoadBalancer(3)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()
	tss.ls.sls = []*lbmpb.ServerList{{
		Servers: []*lbmpb.Server{{
			IpAddress:            tss.beIPs[2],
			Port:                 int32(tss.bePorts[2]),
			LoadBalanceToken:     lbToken,
			DropForLoadBalancing: dropForLoadBalancing,
			DropForRateLimiting:  dropForRateLimiting,
		}},
	}}
	tss.ls.intervals = []time.Duration{0}
	tss.ls.statsDura = 100 * time.Millisecond
	creds := serverNameCheckCreds{expected: besn}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, besn,
		grpc.WithBalancer(grpc.NewGRPCLBBalancer(&testNameResolver{addrs: []string{tss.lbAddr}})),
		grpc.WithTransportCredentials(&creds), grpc.WithPerRPCCredentials(failPreRPCCred{}),
		grpc.WithBlock(), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()

	runRPCs(cc)
	time.Sleep(1 * time.Second)
	tss.ls.mu.Lock()
	stats := tss.ls.stats
	tss.ls.mu.Unlock()
	return stats
}

const countRPC = 40

func TestGRPCLBStatsUnarySuccess(t *testing.T) {
	defer leakcheck.Check(t)
	stats := runAndGetStats(t, false, false, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		// The first non-failfast RPC succeeds, all connections are up.
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
		for i := 0; i < countRPC-1; i++ {
			testC.EmptyCall(context.Background(), &testpb.Empty{})
		}
	})

	if err := checkStats(&stats, &lbmpb.ClientStats{
		NumCallsStarted:               int64(countRPC),
		NumCallsFinished:              int64(countRPC),
		NumCallsFinishedKnownReceived: int64(countRPC),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsUnaryDropLoadBalancing(t *testing.T) {
	defer leakcheck.Check(t)
	c := 0
	stats := runAndGetStats(t, true, false, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		for {
			c++
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
				if strings.Contains(err.Error(), "drops requests") {
					break
				}
			}
		}
		for i := 0; i < countRPC; i++ {
			testC.EmptyCall(context.Background(), &testpb.Empty{})
		}
	})

	if err := checkStats(&stats, &lbmpb.ClientStats{
		NumCallsStarted:                          int64(countRPC + c),
		NumCallsFinished:                         int64(countRPC + c),
		NumCallsFinishedWithDropForLoadBalancing: int64(countRPC + 1),
		NumCallsFinishedWithClientFailedToSend:   int64(c - 1),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsUnaryDropRateLimiting(t *testing.T) {
	defer leakcheck.Check(t)
	c := 0
	stats := runAndGetStats(t, false, true, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		for {
			c++
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
				if strings.Contains(err.Error(), "drops requests") {
					break
				}
			}
		}
		for i := 0; i < countRPC; i++ {
			testC.EmptyCall(context.Background(), &testpb.Empty{})
		}
	})

	if err := checkStats(&stats, &lbmpb.ClientStats{
		NumCallsStarted:                         int64(countRPC + c),
		NumCallsFinished:                        int64(countRPC + c),
		NumCallsFinishedWithDropForRateLimiting: int64(countRPC + 1),
		NumCallsFinishedWithClientFailedToSend:  int64(c - 1),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsUnaryFailedToSend(t *testing.T) {
	defer leakcheck.Check(t)
	stats := runAndGetStats(t, false, false, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		// The first non-failfast RPC succeeds, all connections are up.
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
		for i := 0; i < countRPC-1; i++ {
			grpc.Invoke(context.Background(), "failtosend", &testpb.Empty{}, nil, cc)
		}
	})

	if err := checkStats(&stats, &lbmpb.ClientStats{
		NumCallsStarted:                        int64(countRPC),
		NumCallsFinished:                       int64(countRPC),
		NumCallsFinishedWithClientFailedToSend: int64(countRPC - 1),
		NumCallsFinishedKnownReceived:          1,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsStreamingSuccess(t *testing.T) {
	defer leakcheck.Check(t)
	stats := runAndGetStats(t, false, false, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		// The first non-failfast RPC succeeds, all connections are up.
		stream, err := testC.FullDuplexCall(context.Background(), grpc.FailFast(false))
		if err != nil {
			t.Fatalf("%v.FullDuplexCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
		for {
			if _, err = stream.Recv(); err == io.EOF {
				break
			}
		}
		for i := 0; i < countRPC-1; i++ {
			stream, err = testC.FullDuplexCall(context.Background())
			if err == nil {
				// Wait for stream to end if err is nil.
				for {
					if _, err = stream.Recv(); err == io.EOF {
						break
					}
				}
			}
		}
	})

	if err := checkStats(&stats, &lbmpb.ClientStats{
		NumCallsStarted:               int64(countRPC),
		NumCallsFinished:              int64(countRPC),
		NumCallsFinishedKnownReceived: int64(countRPC),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsStreamingDropLoadBalancing(t *testing.T) {
	defer leakcheck.Check(t)
	c := 0
	stats := runAndGetStats(t, true, false, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		for {
			c++
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
				if strings.Contains(err.Error(), "drops requests") {
					break
				}
			}
		}
		for i := 0; i < countRPC; i++ {
			testC.FullDuplexCall(context.Background())
		}
	})

	if err := checkStats(&stats, &lbmpb.ClientStats{
		NumCallsStarted:                          int64(countRPC + c),
		NumCallsFinished:                         int64(countRPC + c),
		NumCallsFinishedWithDropForLoadBalancing: int64(countRPC + 1),
		NumCallsFinishedWithClientFailedToSend:   int64(c - 1),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsStreamingDropRateLimiting(t *testing.T) {
	defer leakcheck.Check(t)
	c := 0
	stats := runAndGetStats(t, false, true, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		for {
			c++
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
				if strings.Contains(err.Error(), "drops requests") {
					break
				}
			}
		}
		for i := 0; i < countRPC; i++ {
			testC.FullDuplexCall(context.Background())
		}
	})

	if err := checkStats(&stats, &lbmpb.ClientStats{
		NumCallsStarted:                         int64(countRPC + c),
		NumCallsFinished:                        int64(countRPC + c),
		NumCallsFinishedWithDropForRateLimiting: int64(countRPC + 1),
		NumCallsFinishedWithClientFailedToSend:  int64(c - 1),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsStreamingFailedToSend(t *testing.T) {
	defer leakcheck.Check(t)
	stats := runAndGetStats(t, false, false, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		// The first non-failfast RPC succeeds, all connections are up.
		stream, err := testC.FullDuplexCall(context.Background(), grpc.FailFast(false))
		if err != nil {
			t.Fatalf("%v.FullDuplexCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
		for {
			if _, err = stream.Recv(); err == io.EOF {
				break
			}
		}
		for i := 0; i < countRPC-1; i++ {
			grpc.NewClientStream(context.Background(), &grpc.StreamDesc{}, cc, "failtosend")
		}
	})

	if err := checkStats(&stats, &lbmpb.ClientStats{
		NumCallsStarted:                        int64(countRPC),
		NumCallsFinished:                       int64(countRPC),
		NumCallsFinishedWithClientFailedToSend: int64(countRPC - 1),
		NumCallsFinishedKnownReceived:          1,
	}); err != nil {
		t.Fatal(err)
	}
}
