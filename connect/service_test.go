package connect

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

// Assert io.Closer implementation
var _ io.Closer = new(Service)

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
					require.Nil(err)
				}()
				defer testSvr.Close()
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
				require.Nil(err)
				require.IsType(&tls.Conn{}, conn)
			} else {
				require.NotNil(err)
				require.Contains(err.Error(), tt.wantErr)
			}

			if err == nil {
				conn.Close()
			}
		})
	}
}

func TestService_ServerTLSConfig(t *testing.T) {
	// TODO(banks): it's mostly meaningless to test this now since we directly set
	// the tlsCfg in our TestService helper which is all we'd be asserting on here
	// not the actual implementation. Once agent tls fetching is built, it becomes
	// more meaningful to actually verify it's returning the correct config.
}

func TestService_HTTPClient(t *testing.T) {
	require := require.New(t)
	ca := connect.TestCA(t, nil)

	s := TestService(t, "web", ca)

	// Run a test HTTP server
	testSvr := NewTestServer(t, "backend", ca)
	defer testSvr.Close()
	go func() {
		err := testSvr.ServeHTTPS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello, I am Backend"))
		}))
		require.Nil(t, err)
	}()

	// TODO(banks): this will talk http2 on both client and server. I hit some
	// compatibility issues when testing though need to make sure that the http
	// server with our TLSConfig can actually support HTTP/1.1 as well. Could make
	// this a table test with all 4 permutations of client/server http version
	// support.

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
