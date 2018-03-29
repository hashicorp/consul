package connect

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/stretchr/testify/require"
)

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

			s, err := NewService("web", nil)
			require.Nil(err)

			// Force TLSConfig
			s.clientTLSCfg = NewReloadableTLSConfig(TestTLSConfig(t, "web", ca))

			ctx, cancel := context.WithTimeout(context.Background(),
				100*time.Millisecond)
			defer cancel()

			testSvc := NewTestService(t, tt.presentService, ca)
			testSvc.TimeoutHandshake = !tt.handshake

			if tt.accept {
				go func() {
					err := testSvc.Serve()
					require.Nil(err)
				}()
				defer testSvc.Close()
			}

			// Always expect to be connecting to a "DB"
			resolver := &StaticResolver{
				Addr:    testSvc.Addr,
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
