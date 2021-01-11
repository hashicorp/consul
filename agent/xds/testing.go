package xds

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/mitchellh/go-testing-interface"
	"google.golang.org/grpc/metadata"
)

// TestADSStream mocks
// discovery.AggregatedDiscoveryService_StreamAggregatedResourcesServer to allow
// testing ADS handler.
type TestADSStream struct {
	ctx    context.Context
	sendCh chan *envoy_discovery_v3.DiscoveryResponse
	recvCh chan *envoy_discovery_v3.DiscoveryRequest
}

// NewTestADSStream makes a new TestADSStream
func NewTestADSStream(t testing.T, ctx context.Context) *TestADSStream {
	return &TestADSStream{
		ctx:    ctx,
		sendCh: make(chan *envoy_discovery_v3.DiscoveryResponse, 1),
		recvCh: make(chan *envoy_discovery_v3.DiscoveryRequest, 1),
	}
}

// Send implements ADSStream
func (s *TestADSStream) Send(r *envoy_discovery_v3.DiscoveryResponse) error {
	s.sendCh <- r
	return nil
}

// Recv implements ADSStream
func (s *TestADSStream) Recv() (*envoy_discovery_v3.DiscoveryRequest, error) {
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

func stringToEnvoyVersion(vs string) (*envoy_type_v3.SemanticVersion, bool) {
	parts := strings.Split(vs, ".")
	if len(parts) != 3 {
		return nil, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, false
	}

	return &envoy_type_v3.SemanticVersion{
		MajorNumber: uint32(major),
		MinorNumber: uint32(minor),
		Patch:       uint32(patch),
	}, true
}

// SendReq sends a request from the test server.
func (e *TestEnvoy) SendReq(t testing.T, typeURL string, version, nonce uint64) {
	e.Lock()
	defer e.Unlock()

	ev, valid := stringToEnvoyVersion(proxysupport.EnvoyVersions[0])
	if !valid {
		t.Fatal("envoy version is not valid: %s", proxysupport.EnvoyVersions[0])
	}

	req := &envoy_discovery_v3.DiscoveryRequest{
		VersionInfo: hexString(version),
		Node: &envoy_core_v3.Node{
			Id:            e.proxyID,
			Cluster:       e.proxyID,
			UserAgentName: "envoy",
			UserAgentVersionType: &envoy_core_v3.Node_UserAgentBuildVersion{
				UserAgentBuildVersion: &envoy_core_v3.BuildVersion{
					Version: ev,
				},
			},
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
		e.stream = nil
	}
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}
