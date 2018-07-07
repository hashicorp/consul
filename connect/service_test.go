package connect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

// Assert io.Closer implementation
var _ io.Closer = new(Service)

func TestService_Name(t *testing.T) {
	ca := connect.TestCA(t, nil)
	s := TestService(t, "web", ca)
	assert.Equal(t, "web", s.Name())
}

func TestService_Dial(t *testing.T) {
	ca := connect.TestCA(t, nil)

	tests := []struct {
		name           string
		accept         bool
		handshake      bool
		presentService string
		wantErr        string
	}{
		{
			name:           "working",
			accept:         true,
			handshake:      true,
			presentService: "db",
			wantErr:        "",
		},
		{
			name:           "tcp connect fail",
			accept:         false,
			handshake:      false,
			presentService: "db",
			wantErr:        "connection refused",
		},
		{
			name:           "handshake timeout",
			accept:         true,
			handshake:      false,
			presentService: "db",
			wantErr:        "i/o timeout",
		},
		{
			name:           "bad cert",
			accept:         true,
			handshake:      true,
			presentService: "web",
			wantErr:        "peer certificate mismatch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			s := TestService(t, "web", ca)

			ctx, cancel := context.WithTimeout(context.Background(),
				100*time.Millisecond)
			defer cancel()

			testSvr := NewTestServer(t, tt.presentService, ca)
			testSvr.TimeoutHandshake = !tt.handshake

			if tt.accept {
				go func() {
					err := testSvr.Serve()
					require.NoError(err)
				}()
				defer testSvr.Close()
				<-testSvr.Listening
			}

			// Always expect to be connecting to a "DB"
			resolver := &StaticResolver{
				Addr:    testSvr.Addr,
				CertURI: connect.TestSpiffeIDService(t, "db"),
			}

			// All test runs should complete in under 500ms due to the timeout about.
			// Don't wait for whole test run to get stuck.
			testTimeout := 500 * time.Millisecond
			testTimer := time.AfterFunc(testTimeout, func() {
				panic(fmt.Sprintf("test timed out after %s", testTimeout))
			})

			conn, err := s.Dial(ctx, resolver)
			testTimer.Stop()

			if tt.wantErr == "" {
				require.NoError(err)
				require.IsType(&tls.Conn{}, conn)
			} else {
				require.Error(err)
				require.Contains(err.Error(), tt.wantErr)
			}

			if err == nil {
				conn.Close()
			}
		})
	}
}

func TestService_ServerTLSConfig(t *testing.T) {
	require := require.New(t)

	a := agent.NewTestAgent("007", "")
	defer a.Shutdown()
	client := a.Client()
	agent := client.Agent()

	// NewTestAgent setup a CA already by default

	// Register a local agent service with a managed proxy
	reg := &api.AgentServiceRegistration{
		Name: "web",
		Port: 8080,
	}
	err := agent.ServiceRegister(reg)
	require.NoError(err)

	// Now we should be able to create a service that will eventually get it's TLS
	// all by itself!
	service, err := NewService("web", client)
	require.NoError(err)

	// Wait for it to be ready
	select {
	case <-service.ReadyWait():
		// continue with test case below
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for Service.ReadyWait after 1s")
	}

	tlsCfg := service.ServerTLSConfig()

	// Sanity check it has a leaf with the right ServiceID and that validates with
	// the given roots.
	require.NotNil(tlsCfg.GetCertificate)
	leaf, err := tlsCfg.GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(err)
	cert, err := x509.ParseCertificate(leaf.Certificate[0])
	require.NoError(err)
	require.Len(cert.URIs, 1)
	require.True(strings.HasSuffix(cert.URIs[0].String(), "/svc/web"))

	// Verify it as a client would
	err = clientSideVerifier(tlsCfg, leaf.Certificate)
	require.NoError(err)

	// Now test that rotating the root updates
	{
		// Setup a new generated CA
		connect.TestCAConfigSet(t, a, nil)
	}

	// After some time, both root and leaves should be different but both should
	// still be correct.
	oldRootSubjects := bytes.Join(tlsCfg.RootCAs.Subjects(), []byte(", "))
	oldLeafSerial := connect.HexString(cert.SerialNumber.Bytes())
	oldLeafKeyID := connect.HexString(cert.SubjectKeyId)
	retry.Run(t, func(r *retry.R) {
		updatedCfg := service.ServerTLSConfig()

		// Wait until roots are different
		rootSubjects := bytes.Join(updatedCfg.RootCAs.Subjects(), []byte(", "))
		if bytes.Equal(oldRootSubjects, rootSubjects) {
			r.Fatalf("root certificates should have changed, got %s",
				rootSubjects)
		}

		leaf, err := updatedCfg.GetCertificate(&tls.ClientHelloInfo{})
		r.Check(err)
		cert, err := x509.ParseCertificate(leaf.Certificate[0])
		r.Check(err)

		if oldLeafSerial == connect.HexString(cert.SerialNumber.Bytes()) {
			r.Fatalf("leaf certificate should have changed, got serial %s",
				oldLeafSerial)
		}
		if oldLeafKeyID == connect.HexString(cert.SubjectKeyId) {
			r.Fatalf("leaf should have a different key, got matching SubjectKeyID = %s",
				oldLeafKeyID)
		}
	})
}

func TestService_HTTPClient(t *testing.T) {
	ca := connect.TestCA(t, nil)

	s := TestService(t, "web", ca)

	// Run a test HTTP server
	testSvr := NewTestServer(t, "backend", ca)
	defer testSvr.Close()
	go func() {
		err := testSvr.ServeHTTPS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello, I am Backend"))
		}))
		require.NoError(t, err)
	}()
	<-testSvr.Listening

	// Still get connection refused some times so retry on those
	retry.Run(t, func(r *retry.R) {
		// Hook the service resolver to avoid needing full agent setup.
		s.httpResolverFromAddr = func(addr string) (Resolver, error) {
			// Require in this goroutine seems to block causing a timeout on the Get.
			//require.Equal("https://backend.service.consul:443", addr)
			return &StaticResolver{
				Addr:    testSvr.Addr,
				CertURI: connect.TestSpiffeIDService(t, "backend"),
			}, nil
		}

		client := s.HTTPClient()
		client.Timeout = 1 * time.Second

		resp, err := client.Get("https://backend.service.consul/foo")
		r.Check(err)
		defer resp.Body.Close()

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		r.Check(err)

		got := string(bodyBytes)
		want := "Hello, I am Backend"
		if got != want {
			r.Fatalf("got %s, want %s", got, want)
		}
	})
}
