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

package grpclb

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	durationpb "github.com/golang/protobuf/ptypes/duration"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	lbgrpc "google.golang.org/grpc/balancer/grpclb/grpc_lb_v1"
	lbpb "google.golang.org/grpc/balancer/grpclb/grpc_lb_v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/grpclog/glogger"
	"google.golang.org/grpc/internal/leakcheck"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/status"
	testpb "google.golang.org/grpc/test/grpc_testing"
)

var (
	lbServerName = "bar.com"
	beServerName = "foo.com"
	lbToken      = "iamatoken"

	// Resolver replaces localhost with fakeName in Next().
	// Dialer replaces fakeName with localhost when dialing.
	// This will test that custom dialer is passed from Dial to grpclb.
	fakeName = "fake.Name"
)

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

// merge merges the new client stats into current stats.
//
// It's a test-only method. rpcStats is defined in grpclb_picker.
func (s *rpcStats) merge(new *lbpb.ClientStats) {
	atomic.AddInt64(&s.numCallsStarted, new.NumCallsStarted)
	atomic.AddInt64(&s.numCallsFinished, new.NumCallsFinished)
	atomic.AddInt64(&s.numCallsFinishedWithClientFailedToSend, new.NumCallsFinishedWithClientFailedToSend)
	atomic.AddInt64(&s.numCallsFinishedKnownReceived, new.NumCallsFinishedKnownReceived)
	s.mu.Lock()
	for _, perToken := range new.CallsFinishedWithDrop {
		s.numCallsDropped[perToken.LoadBalanceToken] += perToken.NumCalls
	}
	s.mu.Unlock()
}

func mapsEqual(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v1 := range a {
		if v2, ok := b[k]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}

func atomicEqual(a, b *int64) bool {
	return atomic.LoadInt64(a) == atomic.LoadInt64(b)
}

// equal compares two rpcStats.
//
// It's a test-only method. rpcStats is defined in grpclb_picker.
func (s *rpcStats) equal(new *rpcStats) bool {
	if !atomicEqual(&s.numCallsStarted, &new.numCallsStarted) {
		return false
	}
	if !atomicEqual(&s.numCallsFinished, &new.numCallsFinished) {
		return false
	}
	if !atomicEqual(&s.numCallsFinishedWithClientFailedToSend, &new.numCallsFinishedWithClientFailedToSend) {
		return false
	}
	if !atomicEqual(&s.numCallsFinishedKnownReceived, &new.numCallsFinishedKnownReceived) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	new.mu.Lock()
	defer new.mu.Unlock()
	if !mapsEqual(s.numCallsDropped, new.numCallsDropped) {
		return false
	}
	return true
}

type remoteBalancer struct {
	sls       chan *lbpb.ServerList
	statsDura time.Duration
	done      chan struct{}
	stats     *rpcStats
}

func newRemoteBalancer(intervals []time.Duration) *remoteBalancer {
	return &remoteBalancer{
		sls:   make(chan *lbpb.ServerList, 1),
		done:  make(chan struct{}),
		stats: newRPCStats(),
	}
}

func (b *remoteBalancer) stop() {
	close(b.sls)
	close(b.done)
}

func (b *remoteBalancer) BalanceLoad(stream lbgrpc.LoadBalancer_BalanceLoadServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	initReq := req.GetInitialRequest()
	if initReq.Name != beServerName {
		return status.Errorf(codes.InvalidArgument, "invalid service name: %v", initReq.Name)
	}
	resp := &lbpb.LoadBalanceResponse{
		LoadBalanceResponseType: &lbpb.LoadBalanceResponse_InitialResponse{
			InitialResponse: &lbpb.InitialLoadBalanceResponse{
				ClientStatsReportInterval: &durationpb.Duration{
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
				req *lbpb.LoadBalanceRequest
				err error
			)
			if req, err = stream.Recv(); err != nil {
				return
			}
			b.stats.merge(req.GetClientStats())
		}
	}()
	for v := range b.sls {
		resp = &lbpb.LoadBalanceResponse{
			LoadBalanceResponseType: &lbpb.LoadBalanceResponse_ServerList{
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

	addr     string
	fallback bool
}

const testmdkey = "testmd"

func (s *testServer) EmptyCall(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "failed to receive metadata")
	}
	if !s.fallback && (md == nil || md["lb-token"][0] != lbToken) {
		return nil, status.Errorf(codes.Internal, "received unexpected metadata: %v", md)
	}
	grpc.SetTrailer(ctx, metadata.Pairs(testmdkey, s.addr))
	return &testpb.Empty{}, nil
}

func (s *testServer) FullDuplexCall(stream testpb.TestService_FullDuplexCallServer) error {
	return nil
}

func startBackends(sn string, fallback bool, lis ...net.Listener) (servers []*grpc.Server) {
	for _, l := range lis {
		creds := &serverNameCheckCreds{
			sn: sn,
		}
		s := grpc.NewServer(grpc.Creds(creds))
		testpb.RegisterTestServiceServer(s, &testServer{addr: l.Addr().String(), fallback: fallback})
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
	backends := startBackends(beServerName, false, beListeners...)

	// Start a load balancer.
	lbLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		err = fmt.Errorf("Failed to create the listener for the load balancer %v", err)
		return
	}
	lbCreds := &serverNameCheckCreds{
		sn: lbServerName,
	}
	lb = grpc.NewServer(grpc.Creds(lbCreds))
	ls = newRemoteBalancer(nil)
	lbgrpc.RegisterLoadBalancerServer(lb, ls)
	go func() {
		lb.Serve(lbLis)
	}()

	tss = &testServers{
		lbAddr:  fakeName + ":" + strconv.Itoa(lbLis.Addr().(*net.TCPAddr).Port),
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

	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()

	tss, cleanup, err := newLoadBalancer(1)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()

	be := &lbpb.Server{
		IpAddress:        tss.beIPs[0],
		Port:             int32(tss.bePorts[0]),
		LoadBalanceToken: lbToken,
	}
	var bes []*lbpb.Server
	bes = append(bes, be)
	sl := &lbpb.ServerList{
		Servers: bes,
	}
	tss.ls.sls <- sl
	creds := serverNameCheckCreds{
		expected: beServerName,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, r.Scheme()+":///"+beServerName,
		grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)

	r.NewAddress([]resolver.Address{{
		Addr:       tss.lbAddr,
		Type:       resolver.GRPCLB,
		ServerName: lbServerName,
	}})

	if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
	}
}

// The remote balancer sends response with duplicates to grpclb client.
func TestGRPCLBWeighted(t *testing.T) {
	defer leakcheck.Check(t)

	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()

	tss, cleanup, err := newLoadBalancer(2)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()

	beServers := []*lbpb.Server{{
		IpAddress:        tss.beIPs[0],
		Port:             int32(tss.bePorts[0]),
		LoadBalanceToken: lbToken,
	}, {
		IpAddress:        tss.beIPs[1],
		Port:             int32(tss.bePorts[1]),
		LoadBalanceToken: lbToken,
	}}
	portsToIndex := make(map[int]int)
	for i := range beServers {
		portsToIndex[tss.bePorts[i]] = i
	}

	creds := serverNameCheckCreds{
		expected: beServerName,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, r.Scheme()+":///"+beServerName,
		grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)

	r.NewAddress([]resolver.Address{{
		Addr:       tss.lbAddr,
		Type:       resolver.GRPCLB,
		ServerName: lbServerName,
	}})

	sequences := []string{"00101", "00011"}
	for _, seq := range sequences {
		var (
			bes    []*lbpb.Server
			p      peer.Peer
			result string
		)
		for _, s := range seq {
			bes = append(bes, beServers[s-'0'])
		}
		tss.ls.sls <- &lbpb.ServerList{Servers: bes}

		for i := 0; i < 1000; i++ {
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false), grpc.Peer(&p)); err != nil {
				t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
			}
			result += strconv.Itoa(portsToIndex[p.Addr.(*net.TCPAddr).Port])
		}
		// The generated result will be in format of "0010100101".
		if !strings.Contains(result, strings.Repeat(seq, 2)) {
			t.Errorf("got result sequence %q, want patten %q", result, seq)
		}
	}
}

func TestDropRequest(t *testing.T) {
	defer leakcheck.Check(t)

	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()

	tss, cleanup, err := newLoadBalancer(1)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()
	tss.ls.sls <- &lbpb.ServerList{
		Servers: []*lbpb.Server{{
			IpAddress:        tss.beIPs[0],
			Port:             int32(tss.bePorts[0]),
			LoadBalanceToken: lbToken,
			Drop:             false,
		}, {
			Drop: true,
		}},
	}
	creds := serverNameCheckCreds{
		expected: beServerName,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, r.Scheme()+":///"+beServerName,
		grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)

	r.NewAddress([]resolver.Address{{
		Addr:       tss.lbAddr,
		Type:       resolver.GRPCLB,
		ServerName: lbServerName,
	}})

	// Wait for the 1st, non-fail-fast RPC to succeed. This ensures both server
	// connections are made, because the first one has DropForLoadBalancing set
	// to true.
	var i int
	for i = 0; i < 1000; i++ {
		if _, err := testC.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false)); err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if i >= 1000 {
		t.Fatalf("%v.SayHello(_, _) = _, %v, want _, <nil>", testC, err)
	}
	select {
	case <-ctx.Done():
		t.Fatal("timed out", ctx.Err())
	default:
	}
	for _, failfast := range []bool{true, false} {
		for i := 0; i < 3; i++ {
			// Even RPCs should fail, because the 2st backend has
			// DropForLoadBalancing set to true.
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(failfast)); status.Code(err) != codes.Unavailable {
				t.Errorf("%v.EmptyCall(_, _) = _, %v, want _, %s", testC, err, codes.Unavailable)
			}
			// Odd RPCs should succeed since they choose the non-drop-request
			// backend according to the round robin policy.
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(failfast)); err != nil {
				t.Errorf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
			}
		}
	}
}

// When the balancer in use disconnects, grpclb should connect to the next address from resolved balancer address list.
func TestBalancerDisconnects(t *testing.T) {
	defer leakcheck.Check(t)

	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()

	var (
		tests []*testServers
		lbs   []*grpc.Server
	)
	for i := 0; i < 2; i++ {
		tss, cleanup, err := newLoadBalancer(1)
		if err != nil {
			t.Fatalf("failed to create new load balancer: %v", err)
		}
		defer cleanup()

		be := &lbpb.Server{
			IpAddress:        tss.beIPs[0],
			Port:             int32(tss.bePorts[0]),
			LoadBalanceToken: lbToken,
		}
		var bes []*lbpb.Server
		bes = append(bes, be)
		sl := &lbpb.ServerList{
			Servers: bes,
		}
		tss.ls.sls <- sl

		tests = append(tests, tss)
		lbs = append(lbs, tss.lb)
	}

	creds := serverNameCheckCreds{
		expected: beServerName,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, r.Scheme()+":///"+beServerName,
		grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)

	r.NewAddress([]resolver.Address{{
		Addr:       tests[0].lbAddr,
		Type:       resolver.GRPCLB,
		ServerName: lbServerName,
	}, {
		Addr:       tests[1].lbAddr,
		Type:       resolver.GRPCLB,
		ServerName: lbServerName,
	}})

	var p peer.Peer
	if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false), grpc.Peer(&p)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
	}
	if p.Addr.(*net.TCPAddr).Port != tests[0].bePorts[0] {
		t.Fatalf("got peer: %v, want peer port: %v", p.Addr, tests[0].bePorts[0])
	}

	lbs[0].Stop()
	// Stop balancer[0], balancer[1] should be used by grpclb.
	// Check peer address to see if that happened.
	for i := 0; i < 1000; i++ {
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false), grpc.Peer(&p)); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
		if p.Addr.(*net.TCPAddr).Port == tests[1].bePorts[0] {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("No RPC sent to second backend after 1 second")
}

type customGRPCLBBuilder struct {
	balancer.Builder
	name string
}

func (b *customGRPCLBBuilder) Name() string {
	return b.name
}

const grpclbCustomFallbackName = "grpclb_with_custom_fallback_timeout"

func init() {
	balancer.Register(&customGRPCLBBuilder{
		Builder: newLBBuilderWithFallbackTimeout(100 * time.Millisecond),
		name:    grpclbCustomFallbackName,
	})
}

func TestFallback(t *testing.T) {
	defer leakcheck.Check(t)

	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()

	tss, cleanup, err := newLoadBalancer(1)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()

	// Start a standalone backend.
	beLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen %v", err)
	}
	defer beLis.Close()
	standaloneBEs := startBackends(beServerName, true, beLis)
	defer stopBackends(standaloneBEs)

	be := &lbpb.Server{
		IpAddress:        tss.beIPs[0],
		Port:             int32(tss.bePorts[0]),
		LoadBalanceToken: lbToken,
	}
	var bes []*lbpb.Server
	bes = append(bes, be)
	sl := &lbpb.ServerList{
		Servers: bes,
	}
	tss.ls.sls <- sl
	creds := serverNameCheckCreds{
		expected: beServerName,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, r.Scheme()+":///"+beServerName,
		grpc.WithBalancerName(grpclbCustomFallbackName),
		grpc.WithTransportCredentials(&creds), grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()
	testC := testpb.NewTestServiceClient(cc)

	r.NewAddress([]resolver.Address{{
		Addr:       "",
		Type:       resolver.GRPCLB,
		ServerName: lbServerName,
	}, {
		Addr:       beLis.Addr().String(),
		Type:       resolver.Backend,
		ServerName: beServerName,
	}})

	var p peer.Peer
	if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false), grpc.Peer(&p)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
	}
	if p.Addr.String() != beLis.Addr().String() {
		t.Fatalf("got peer: %v, want peer: %v", p.Addr, beLis.Addr())
	}

	r.NewAddress([]resolver.Address{{
		Addr:       tss.lbAddr,
		Type:       resolver.GRPCLB,
		ServerName: lbServerName,
	}, {
		Addr:       beLis.Addr().String(),
		Type:       resolver.Backend,
		ServerName: beServerName,
	}})

	for i := 0; i < 1000; i++ {
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false), grpc.Peer(&p)); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
		if p.Addr.(*net.TCPAddr).Port == tss.bePorts[0] {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("No RPC sent to backend behind remote balancer after 1 second")
}

type failPreRPCCred struct{}

func (failPreRPCCred) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	if strings.Contains(uri[0], failtosendURI) {
		return nil, fmt.Errorf("rpc should fail to send")
	}
	return nil, nil
}

func (failPreRPCCred) RequireTransportSecurity() bool {
	return false
}

func checkStats(stats, expected *rpcStats) error {
	if !stats.equal(expected) {
		return fmt.Errorf("stats not equal: got %+v, want %+v", stats, expected)
	}
	return nil
}

func runAndGetStats(t *testing.T, drop bool, runRPCs func(*grpc.ClientConn)) *rpcStats {
	defer leakcheck.Check(t)

	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()

	tss, cleanup, err := newLoadBalancer(1)
	if err != nil {
		t.Fatalf("failed to create new load balancer: %v", err)
	}
	defer cleanup()
	tss.ls.sls <- &lbpb.ServerList{
		Servers: []*lbpb.Server{{
			IpAddress:        tss.beIPs[0],
			Port:             int32(tss.bePorts[0]),
			LoadBalanceToken: lbToken,
			Drop:             drop,
		}},
	}
	tss.ls.statsDura = 100 * time.Millisecond
	creds := serverNameCheckCreds{expected: beServerName}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, r.Scheme()+":///"+beServerName,
		grpc.WithTransportCredentials(&creds),
		grpc.WithPerRPCCredentials(failPreRPCCred{}),
		grpc.WithDialer(fakeNameDialer))
	if err != nil {
		t.Fatalf("Failed to dial to the backend %v", err)
	}
	defer cc.Close()

	r.NewAddress([]resolver.Address{{
		Addr:       tss.lbAddr,
		Type:       resolver.GRPCLB,
		ServerName: lbServerName,
	}})

	runRPCs(cc)
	time.Sleep(1 * time.Second)
	stats := tss.ls.stats
	return stats
}

const (
	countRPC      = 40
	failtosendURI = "failtosend"
	dropErrDesc   = "request dropped by grpclb"
)

func TestGRPCLBStatsUnarySuccess(t *testing.T) {
	defer leakcheck.Check(t)
	stats := runAndGetStats(t, false, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		// The first non-failfast RPC succeeds, all connections are up.
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
		for i := 0; i < countRPC-1; i++ {
			testC.EmptyCall(context.Background(), &testpb.Empty{})
		}
	})

	if err := checkStats(stats, &rpcStats{
		numCallsStarted:               int64(countRPC),
		numCallsFinished:              int64(countRPC),
		numCallsFinishedKnownReceived: int64(countRPC),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsUnaryDrop(t *testing.T) {
	defer leakcheck.Check(t)
	c := 0
	stats := runAndGetStats(t, true, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		for {
			c++
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
				if strings.Contains(err.Error(), dropErrDesc) {
					break
				}
			}
		}
		for i := 0; i < countRPC; i++ {
			testC.EmptyCall(context.Background(), &testpb.Empty{})
		}
	})

	if err := checkStats(stats, &rpcStats{
		numCallsStarted:                        int64(countRPC + c),
		numCallsFinished:                       int64(countRPC + c),
		numCallsFinishedWithClientFailedToSend: int64(c - 1),
		numCallsDropped:                        map[string]int64{lbToken: int64(countRPC + 1)},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsUnaryFailedToSend(t *testing.T) {
	defer leakcheck.Check(t)
	stats := runAndGetStats(t, false, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		// The first non-failfast RPC succeeds, all connections are up.
		if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
			t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, <nil>", testC, err)
		}
		for i := 0; i < countRPC-1; i++ {
			cc.Invoke(context.Background(), failtosendURI, &testpb.Empty{}, nil)
		}
	})

	if err := checkStats(stats, &rpcStats{
		numCallsStarted:                        int64(countRPC),
		numCallsFinished:                       int64(countRPC),
		numCallsFinishedWithClientFailedToSend: int64(countRPC - 1),
		numCallsFinishedKnownReceived:          1,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsStreamingSuccess(t *testing.T) {
	defer leakcheck.Check(t)
	stats := runAndGetStats(t, false, func(cc *grpc.ClientConn) {
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

	if err := checkStats(stats, &rpcStats{
		numCallsStarted:               int64(countRPC),
		numCallsFinished:              int64(countRPC),
		numCallsFinishedKnownReceived: int64(countRPC),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsStreamingDrop(t *testing.T) {
	defer leakcheck.Check(t)
	c := 0
	stats := runAndGetStats(t, true, func(cc *grpc.ClientConn) {
		testC := testpb.NewTestServiceClient(cc)
		for {
			c++
			if _, err := testC.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
				if strings.Contains(err.Error(), dropErrDesc) {
					break
				}
			}
		}
		for i := 0; i < countRPC; i++ {
			testC.FullDuplexCall(context.Background())
		}
	})

	if err := checkStats(stats, &rpcStats{
		numCallsStarted:                        int64(countRPC + c),
		numCallsFinished:                       int64(countRPC + c),
		numCallsFinishedWithClientFailedToSend: int64(c - 1),
		numCallsDropped:                        map[string]int64{lbToken: int64(countRPC + 1)},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCLBStatsStreamingFailedToSend(t *testing.T) {
	defer leakcheck.Check(t)
	stats := runAndGetStats(t, false, func(cc *grpc.ClientConn) {
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
			cc.NewStream(context.Background(), &grpc.StreamDesc{}, failtosendURI)
		}
	})

	if err := checkStats(stats, &rpcStats{
		numCallsStarted:                        int64(countRPC),
		numCallsFinished:                       int64(countRPC),
		numCallsFinishedWithClientFailedToSend: int64(countRPC - 1),
		numCallsFinishedKnownReceived:          1,
	}); err != nil {
		t.Fatal(err)
	}
}
