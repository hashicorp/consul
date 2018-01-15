/*
 *
 * Copyright 2014 gRPC authors.
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

package grpc

import (
	"io"
	"math"
	"net"
	"testing"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/http2"

	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/naming"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	_ "google.golang.org/grpc/resolver/passthrough"
	"google.golang.org/grpc/test/leakcheck"
	"google.golang.org/grpc/testdata"
)

func assertState(wantState connectivity.State, cc *ClientConn) (connectivity.State, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var state connectivity.State
	for state = cc.GetState(); state != wantState && cc.WaitForStateChange(ctx, state); state = cc.GetState() {
	}
	return state, state == wantState
}

func TestDialWithMultipleBackendsNotSendingServerPreface(t *testing.T) {
	defer leakcheck.Check(t)
	numServers := 2
	servers := make([]net.Listener, numServers)
	var err error
	for i := 0; i < numServers; i++ {
		servers[i], err = net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Error while listening. Err: %v", err)
		}
	}
	dones := make([]chan struct{}, numServers)
	for i := 0; i < numServers; i++ {
		dones[i] = make(chan struct{})
	}
	for i := 0; i < numServers; i++ {
		go func(i int) {
			defer func() {
				close(dones[i])
			}()
			conn, err := servers[i].Accept()
			if err != nil {
				t.Errorf("Error while accepting. Err: %v", err)
				return
			}
			defer conn.Close()
			switch i {
			case 0: // 1st server accepts the connection and immediately closes it.
			case 1: // 2nd server accepts the connection and sends settings frames.
				framer := http2.NewFramer(conn, conn)
				if err := framer.WriteSettings(http2.Setting{}); err != nil {
					t.Errorf("Error while writing settings frame. %v", err)
					return
				}
				conn.SetDeadline(time.Now().Add(time.Second))
				buf := make([]byte, 1024)
				for { // Make sure the connection stays healthy.
					_, err = conn.Read(buf)
					if err == nil {
						continue
					}
					if nerr, ok := err.(net.Error); !ok || !nerr.Timeout() {
						t.Errorf("Server expected the conn.Read(_) to timeout instead got error: %v", err)
					}
					return
				}
			}
		}(i)
	}
	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()
	resolvedAddrs := make([]resolver.Address, numServers)
	for i := 0; i < numServers; i++ {
		resolvedAddrs[i] = resolver.Address{Addr: servers[i].Addr().String()}
	}
	r.InitialAddrs(resolvedAddrs)
	client, err := Dial(r.Scheme()+":///test.server", WithInsecure())
	if err != nil {
		t.Errorf("Dial failed. Err: %v", err)
	} else {
		defer client.Close()
	}
	time.Sleep(time.Second) // Close the servers after a second for cleanup.
	for _, s := range servers {
		s.Close()
	}
	for _, done := range dones {
		<-done
	}
}

func TestDialWaitsForServerSettings(t *testing.T) {
	defer leakcheck.Check(t)
	server, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Error while listening. Err: %v", err)
	}
	defer server.Close()
	done := make(chan struct{})
	sent := make(chan struct{})
	dialDone := make(chan struct{})
	go func() { // Launch the server.
		defer func() {
			close(done)
		}()
		conn, err := server.Accept()
		if err != nil {
			t.Errorf("Error while accepting. Err: %v", err)
			return
		}
		defer conn.Close()
		// Sleep so that if the test were to fail it
		// will fail more often than not.
		time.Sleep(100 * time.Millisecond)
		framer := http2.NewFramer(conn, conn)
		close(sent)
		if err := framer.WriteSettings(http2.Setting{}); err != nil {
			t.Errorf("Error while writing settings. Err: %v", err)
			return
		}
		<-dialDone // Close conn only after dial returns.
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	client, err := DialContext(ctx, server.Addr().String(), WithInsecure(), WithWaitForHandshake(), WithBlock())
	close(dialDone)
	if err != nil {
		cancel()
		t.Fatalf("Error while dialing. Err: %v", err)
	}
	defer client.Close()
	select {
	case <-sent:
	default:
		t.Fatalf("Dial returned before server settings were sent")
	}
	<-done

}

func TestCloseConnectionWhenServerPrefaceNotReceived(t *testing.T) {
	mctBkp := minConnectTimeout
	// Call this only after transportMonitor goroutine has ended.
	defer func() {
		minConnectTimeout = mctBkp
	}()
	defer leakcheck.Check(t)
	minConnectTimeout = time.Millisecond * 500
	server, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Error while listening. Err: %v", err)
	}
	defer server.Close()
	done := make(chan struct{})
	clientDone := make(chan struct{})
	go func() { // Launch the server.
		defer func() {
			if done != nil {
				close(done)
			}
		}()
		conn1, err := server.Accept()
		if err != nil {
			t.Errorf("Error while accepting. Err: %v", err)
			return
		}
		defer conn1.Close()
		// Don't send server settings and make sure the connection is closed.
		time.Sleep(time.Millisecond * 1500) // Since the first backoff is for a second.
		conn1.SetDeadline(time.Now().Add(time.Second))
		b := make([]byte, 24)
		for {
			// Make sure the connection was closed by client.
			_, err = conn1.Read(b)
			if err == nil {
				continue
			}
			if err != io.EOF {
				t.Errorf(" conn1.Read(_) = _, %v, want _, io.EOF", err)
				return
			}
			break
		}

		conn2, err := server.Accept() // Accept a reconnection request from client.
		if err != nil {
			t.Errorf("Error while accepting. Err: %v", err)
			return
		}
		defer conn2.Close()
		framer := http2.NewFramer(conn2, conn2)
		if err := framer.WriteSettings(http2.Setting{}); err != nil {
			t.Errorf("Error while writing settings. Err: %v", err)
			return
		}
		time.Sleep(time.Millisecond * 1500) // Since the first backoff is for a second.
		conn2.SetDeadline(time.Now().Add(time.Millisecond * 500))
		for {
			// Make sure the connection stays open and is closed
			// only by connection timeout.
			_, err = conn2.Read(b)
			if err == nil {
				continue
			}
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				return
			}
			t.Errorf("Unexpected error while reading. Err: %v, want timeout error", err)
			break
		}
		close(done)
		done = nil
		<-clientDone

	}()
	client, err := Dial(server.Addr().String(), WithInsecure())
	if err != nil {
		t.Fatalf("Error while dialing. Err: %v", err)
	}
	<-done
	// TODO: The code from BEGIN to END should be delete once issue
	// https://github.com/grpc/grpc-go/issues/1750 is fixed.
	// BEGIN
	// Set underlying addrConns state to Shutdown so that no reconnect
	// attempts take place and thereby resetting minConnectTimeout is
	// race free.
	client.mu.Lock()
	addrConns := client.conns
	client.mu.Unlock()
	for ac := range addrConns {
		ac.mu.Lock()
		ac.state = connectivity.Shutdown
		ac.mu.Unlock()
	}
	// END
	client.Close()
	close(clientDone)
}

func TestBackoffWhenNoServerPrefaceReceived(t *testing.T) {
	defer leakcheck.Check(t)
	server, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Error while listening. Err: %v", err)
	}
	defer server.Close()
	done := make(chan struct{})
	go func() { // Launch the server.
		defer func() {
			close(done)
		}()
		conn, err := server.Accept() // Accept the connection only to close it immediately.
		if err != nil {
			t.Errorf("Error while accepting. Err: %v", err)
			return
		}
		prevAt := time.Now()
		conn.Close()
		var prevDuration time.Duration
		// Make sure the retry attempts are backed off properly.
		for i := 0; i < 3; i++ {
			conn, err := server.Accept()
			if err != nil {
				t.Errorf("Error while accepting. Err: %v", err)
				return
			}
			meow := time.Now()
			conn.Close()
			dr := meow.Sub(prevAt)
			if dr <= prevDuration {
				t.Errorf("Client backoff did not increase with retries. Previous duration: %v, current duration: %v", prevDuration, dr)
				return
			}
			prevDuration = dr
			prevAt = meow
		}
	}()
	client, err := Dial(server.Addr().String(), WithInsecure())
	if err != nil {
		t.Fatalf("Error while dialing. Err: %v", err)
	}
	defer client.Close()
	<-done

}

func TestConnectivityStates(t *testing.T) {
	defer leakcheck.Check(t)
	servers, resolver, cleanup := startServers(t, 2, math.MaxUint32)
	defer cleanup()
	cc, err := Dial("passthrough:///foo.bar.com", WithBalancer(RoundRobin(resolver)), WithInsecure())
	if err != nil {
		t.Fatalf("Dial(\"foo.bar.com\", WithBalancer(_)) = _, %v, want _ <nil>", err)
	}
	defer cc.Close()
	wantState := connectivity.Ready
	if state, ok := assertState(wantState, cc); !ok {
		t.Fatalf("asserState(%s) = %s, false, want %s, true", wantState, state, wantState)
	}
	// Send an update to delete the server connection (tearDown addrConn).
	update := []*naming.Update{
		{
			Op:   naming.Delete,
			Addr: "localhost:" + servers[0].port,
		},
	}
	resolver.w.inject(update)
	wantState = connectivity.TransientFailure
	if state, ok := assertState(wantState, cc); !ok {
		t.Fatalf("asserState(%s) = %s, false, want %s, true", wantState, state, wantState)
	}
	update[0] = &naming.Update{
		Op:   naming.Add,
		Addr: "localhost:" + servers[1].port,
	}
	resolver.w.inject(update)
	wantState = connectivity.Ready
	if state, ok := assertState(wantState, cc); !ok {
		t.Fatalf("asserState(%s) = %s, false, want %s, true", wantState, state, wantState)
	}

}

func TestDialTimeout(t *testing.T) {
	defer leakcheck.Check(t)
	conn, err := Dial("passthrough:///Non-Existent.Server:80", WithTimeout(time.Millisecond), WithBlock(), WithInsecure())
	if err == nil {
		conn.Close()
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("Dial(_, _) = %v, %v, want %v", conn, err, context.DeadlineExceeded)
	}
}

func TestTLSDialTimeout(t *testing.T) {
	defer leakcheck.Check(t)
	creds, err := credentials.NewClientTLSFromFile(testdata.Path("ca.pem"), "x.test.youtube.com")
	if err != nil {
		t.Fatalf("Failed to create credentials %v", err)
	}
	conn, err := Dial("passthrough:///Non-Existent.Server:80", WithTransportCredentials(creds), WithTimeout(time.Millisecond), WithBlock())
	if err == nil {
		conn.Close()
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("Dial(_, _) = %v, %v, want %v", conn, err, context.DeadlineExceeded)
	}
}

func TestDefaultAuthority(t *testing.T) {
	defer leakcheck.Check(t)
	target := "Non-Existent.Server:8080"
	conn, err := Dial(target, WithInsecure())
	if err != nil {
		t.Fatalf("Dial(_, _) = _, %v, want _, <nil>", err)
	}
	defer conn.Close()
	if conn.authority != target {
		t.Fatalf("%v.authority = %v, want %v", conn, conn.authority, target)
	}
}

func TestTLSServerNameOverwrite(t *testing.T) {
	defer leakcheck.Check(t)
	overwriteServerName := "over.write.server.name"
	creds, err := credentials.NewClientTLSFromFile(testdata.Path("ca.pem"), overwriteServerName)
	if err != nil {
		t.Fatalf("Failed to create credentials %v", err)
	}
	conn, err := Dial("passthrough:///Non-Existent.Server:80", WithTransportCredentials(creds))
	if err != nil {
		t.Fatalf("Dial(_, _) = _, %v, want _, <nil>", err)
	}
	defer conn.Close()
	if conn.authority != overwriteServerName {
		t.Fatalf("%v.authority = %v, want %v", conn, conn.authority, overwriteServerName)
	}
}

func TestWithAuthority(t *testing.T) {
	defer leakcheck.Check(t)
	overwriteServerName := "over.write.server.name"
	conn, err := Dial("passthrough:///Non-Existent.Server:80", WithInsecure(), WithAuthority(overwriteServerName))
	if err != nil {
		t.Fatalf("Dial(_, _) = _, %v, want _, <nil>", err)
	}
	defer conn.Close()
	if conn.authority != overwriteServerName {
		t.Fatalf("%v.authority = %v, want %v", conn, conn.authority, overwriteServerName)
	}
}

func TestWithAuthorityAndTLS(t *testing.T) {
	defer leakcheck.Check(t)
	overwriteServerName := "over.write.server.name"
	creds, err := credentials.NewClientTLSFromFile(testdata.Path("ca.pem"), overwriteServerName)
	if err != nil {
		t.Fatalf("Failed to create credentials %v", err)
	}
	conn, err := Dial("passthrough:///Non-Existent.Server:80", WithTransportCredentials(creds), WithAuthority("no.effect.authority"))
	if err != nil {
		t.Fatalf("Dial(_, _) = _, %v, want _, <nil>", err)
	}
	defer conn.Close()
	if conn.authority != overwriteServerName {
		t.Fatalf("%v.authority = %v, want %v", conn, conn.authority, overwriteServerName)
	}
}

func TestDialContextCancel(t *testing.T) {
	defer leakcheck.Check(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DialContext(ctx, "Non-Existent.Server:80", WithBlock(), WithInsecure()); err != context.Canceled {
		t.Fatalf("DialContext(%v, _) = _, %v, want _, %v", ctx, err, context.Canceled)
	}
}

// blockingBalancer mimics the behavior of balancers whose initialization takes a long time.
// In this test, reading from blockingBalancer.Notify() blocks forever.
type blockingBalancer struct {
	ch chan []Address
}

func newBlockingBalancer() Balancer {
	return &blockingBalancer{ch: make(chan []Address)}
}
func (b *blockingBalancer) Start(target string, config BalancerConfig) error {
	return nil
}
func (b *blockingBalancer) Up(addr Address) func(error) {
	return nil
}
func (b *blockingBalancer) Get(ctx context.Context, opts BalancerGetOptions) (addr Address, put func(), err error) {
	return Address{}, nil, nil
}
func (b *blockingBalancer) Notify() <-chan []Address {
	return b.ch
}
func (b *blockingBalancer) Close() error {
	close(b.ch)
	return nil
}

func TestDialWithBlockingBalancer(t *testing.T) {
	defer leakcheck.Check(t)
	ctx, cancel := context.WithCancel(context.Background())
	dialDone := make(chan struct{})
	go func() {
		DialContext(ctx, "Non-Existent.Server:80", WithBlock(), WithInsecure(), WithBalancer(newBlockingBalancer()))
		close(dialDone)
	}()
	cancel()
	<-dialDone
}

// securePerRPCCredentials always requires transport security.
type securePerRPCCredentials struct{}

func (c securePerRPCCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return nil, nil
}

func (c securePerRPCCredentials) RequireTransportSecurity() bool {
	return true
}

func TestCredentialsMisuse(t *testing.T) {
	defer leakcheck.Check(t)
	tlsCreds, err := credentials.NewClientTLSFromFile(testdata.Path("ca.pem"), "x.test.youtube.com")
	if err != nil {
		t.Fatalf("Failed to create authenticator %v", err)
	}
	// Two conflicting credential configurations
	if _, err := Dial("passthrough:///Non-Existent.Server:80", WithTransportCredentials(tlsCreds), WithBlock(), WithInsecure()); err != errCredentialsConflict {
		t.Fatalf("Dial(_, _) = _, %v, want _, %v", err, errCredentialsConflict)
	}
	// security info on insecure connection
	if _, err := Dial("passthrough:///Non-Existent.Server:80", WithPerRPCCredentials(securePerRPCCredentials{}), WithBlock(), WithInsecure()); err != errTransportCredentialsMissing {
		t.Fatalf("Dial(_, _) = _, %v, want _, %v", err, errTransportCredentialsMissing)
	}
}

func TestWithBackoffConfigDefault(t *testing.T) {
	defer leakcheck.Check(t)
	testBackoffConfigSet(t, &DefaultBackoffConfig)
}

func TestWithBackoffConfig(t *testing.T) {
	defer leakcheck.Check(t)
	b := BackoffConfig{MaxDelay: DefaultBackoffConfig.MaxDelay / 2}
	expected := b
	setDefaults(&expected) // defaults should be set
	testBackoffConfigSet(t, &expected, WithBackoffConfig(b))
}

func TestWithBackoffMaxDelay(t *testing.T) {
	defer leakcheck.Check(t)
	md := DefaultBackoffConfig.MaxDelay / 2
	expected := BackoffConfig{MaxDelay: md}
	setDefaults(&expected)
	testBackoffConfigSet(t, &expected, WithBackoffMaxDelay(md))
}

func testBackoffConfigSet(t *testing.T, expected *BackoffConfig, opts ...DialOption) {
	opts = append(opts, WithInsecure())
	conn, err := Dial("passthrough:///foo:80", opts...)
	if err != nil {
		t.Fatalf("unexpected error dialing connection: %v", err)
	}
	defer conn.Close()

	if conn.dopts.bs == nil {
		t.Fatalf("backoff config not set")
	}

	actual, ok := conn.dopts.bs.(BackoffConfig)
	if !ok {
		t.Fatalf("unexpected type of backoff config: %#v", conn.dopts.bs)
	}

	if actual != *expected {
		t.Fatalf("unexpected backoff config on connection: %v, want %v", actual, expected)
	}
}

// emptyBalancer returns an empty set of servers.
type emptyBalancer struct {
	ch chan []Address
}

func newEmptyBalancer() Balancer {
	return &emptyBalancer{ch: make(chan []Address, 1)}
}
func (b *emptyBalancer) Start(_ string, _ BalancerConfig) error {
	b.ch <- nil
	return nil
}
func (b *emptyBalancer) Up(_ Address) func(error) {
	return nil
}
func (b *emptyBalancer) Get(_ context.Context, _ BalancerGetOptions) (Address, func(), error) {
	return Address{}, nil, nil
}
func (b *emptyBalancer) Notify() <-chan []Address {
	return b.ch
}
func (b *emptyBalancer) Close() error {
	close(b.ch)
	return nil
}

func TestNonblockingDialWithEmptyBalancer(t *testing.T) {
	defer leakcheck.Check(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dialDone := make(chan error)
	go func() {
		dialDone <- func() error {
			conn, err := DialContext(ctx, "Non-Existent.Server:80", WithInsecure(), WithBalancer(newEmptyBalancer()))
			if err != nil {
				return err
			}
			return conn.Close()
		}()
	}()
	if err := <-dialDone; err != nil {
		t.Fatalf("unexpected error dialing connection: %s", err)
	}
}

func TestClientUpdatesParamsAfterGoAway(t *testing.T) {
	defer leakcheck.Check(t)
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen. Err: %v", err)
	}
	defer lis.Close()
	addr := lis.Addr().String()
	s := NewServer()
	go s.Serve(lis)
	defer s.Stop()
	cc, err := Dial(addr, WithBlock(), WithInsecure(), WithKeepaliveParams(keepalive.ClientParameters{
		Time:                50 * time.Millisecond,
		Timeout:             100 * time.Millisecond,
		PermitWithoutStream: true,
	}))
	if err != nil {
		t.Fatalf("Dial(%s, _) = _, %v, want _, <nil>", addr, err)
	}
	defer cc.Close()
	time.Sleep(1 * time.Second)
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	v := cc.mkp.Time
	if v < 100*time.Millisecond {
		t.Fatalf("cc.dopts.copts.Keepalive.Time = %v , want 100ms", v)
	}
}
