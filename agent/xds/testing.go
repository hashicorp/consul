package xds

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyauth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2"
	"github.com/mitchellh/go-testing-interface"
	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/agent/connect"
)

// TestADSStream mocks
// discovery.AggregatedDiscoveryService_StreamAggregatedResourcesServer to allow
// testing ADS handler.
type TestADSStream struct {
	ctx    context.Context
	sendCh chan *envoy.DiscoveryResponse
	recvCh chan *envoy.DiscoveryRequest
}

// NewTestADSStream makes a new TestADSStream
func NewTestADSStream(t testing.T, ctx context.Context) *TestADSStream {
	return &TestADSStream{
		ctx:    ctx,
		sendCh: make(chan *envoy.DiscoveryResponse, 1),
		recvCh: make(chan *envoy.DiscoveryRequest, 1),
	}
}

// Send implements ADSStream
func (s *TestADSStream) Send(r *envoy.DiscoveryResponse) error {
	s.sendCh <- r
	return nil
}

// Recv implements ADSStream
func (s *TestADSStream) Recv() (*envoy.DiscoveryRequest, error) {
	r := <-s.recvCh
	if r == nil {
		return nil, io.EOF
	}
	return r, nil
}

// SetHeader implements ADSStream
func (s *TestADSStream) SetHeader(metadata.MD) error {
	return nil
}

// SendHeader implements ADSStream
func (s *TestADSStream) SendHeader(metadata.MD) error {
	return nil
}

// SetTrailer implements ADSStream
func (s *TestADSStream) SetTrailer(metadata.MD) {
}

// Context implements ADSStream
func (s *TestADSStream) Context() context.Context {
	return s.ctx
}

// SendMsg implements ADSStream
func (s *TestADSStream) SendMsg(m interface{}) error {
	return nil
}

// RecvMsg implements ADSStream
func (s *TestADSStream) RecvMsg(m interface{}) error {
	return nil
}

type configState struct {
	lastNonce, lastVersion, acceptedVersion string
}

// TestEnvoy is a helper to simulate Envoy ADS requests.
type TestEnvoy struct {
	sync.Mutex
	stream  *TestADSStream
	proxyID string
	token   string
	state   map[string]configState
	ctx     context.Context
	cancel  func()
}

// NewTestEnvoy creates a TestEnvoy instance.
func NewTestEnvoy(t testing.T, proxyID, token string) *TestEnvoy {
	ctx, cancel := context.WithCancel(context.Background())
	// If a token is given, attach it to the context in the same way gRPC attaches
	// metadata in calls and stream contexts.
	if token != "" {
		ctx = metadata.NewIncomingContext(ctx,
			metadata.Pairs("x-consul-token", token))
	}
	return &TestEnvoy{
		stream:  NewTestADSStream(t, ctx),
		state:   make(map[string]configState),
		ctx:     ctx,
		cancel:  cancel,
		proxyID: proxyID,
		token:   token,
	}
}

func hexString(v uint64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%08x", v)
}

// SendReq sends a request from the test server.
func (e *TestEnvoy) SendReq(t testing.T, typeURL string, version, nonce uint64) {
	e.Lock()
	defer e.Unlock()

	req := &envoy.DiscoveryRequest{
		VersionInfo: hexString(version),
		Node: &envoycore.Node{
			Id:      e.proxyID,
			Cluster: e.proxyID,
		},
		ResponseNonce: hexString(nonce),
		TypeUrl:       typeURL,
	}
	select {
	case e.stream.recvCh <- req:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("send to stream blocked for too long")
	}
}

// Close closes the client and cancels it's request context.
func (e *TestEnvoy) Close() error {
	e.Lock()
	defer e.Unlock()

	// unblock the recv chan to simulate recv error when client disconnects
	if e.stream != nil && e.stream.recvCh != nil {
		close(e.stream.recvCh)
		e.stream.recvCh = nil
	}
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}

// TestCheckRequest creates an envoyauth.CheckRequest with the source and
// destination service names.
func TestCheckRequest(t testing.T, source, dest string) *envoyauth.CheckRequest {
	return &envoyauth.CheckRequest{
		Attributes: &envoyauth.AttributeContext{
			Source:      makeAttributeContextPeer(t, source),
			Destination: makeAttributeContextPeer(t, dest),
		},
	}
}

func makeAttributeContextPeer(t testing.T, svc string) *envoyauth.AttributeContext_Peer {
	spiffeID := connect.TestSpiffeIDService(t, svc)
	return &envoyauth.AttributeContext_Peer{
		// We don't care about IP for now might later though
		Address: makeAddressPtr("10.0.0.1", 1234),
		// Note we don't set Service since that is an advisory only mechanism in
		// Envoy triggered by self-declared headers. We rely on the actual TLS Peer
		// identity.
		Principal: spiffeID.URI().String(),
	}
}
