// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

// Assert io.Closer implementation
var _ io.Closer = new(Service)

func TestService_Name(t *testing.T) {
	ca := connect.TestCA(t, nil)
	s := TestService(t, "web", ca)
	assert.Equal(t, "web", s.Name())
}

func TestService_Dial(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

			s := TestService(t, "web", ca)

			ctx, cancel := context.WithTimeout(context.Background(),
				100*time.Millisecond)
			defer cancel()

			testSvr := NewTestServer(t, tt.presentService, ca)
			testSvr.TimeoutHandshake = !tt.handshake

			if tt.accept {
				go func() {
					err := testSvr.Serve()
					require.NoError(t, err)
				}()
				<-testSvr.Listening
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
				require.NoError(t, err)
				require.IsType(t, &tls.Conn{}, conn)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}

			if err == nil {
				conn.Close()
			}
		})
	}
}

func TestService_ServerTLSConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := agent.StartTestAgent(t, agent.TestAgent{Name: "007", Overrides: `
		connect {
			test_ca_leaf_root_change_spread = "1ns"
		}
	`})
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	client := a.Client()
	agent := client.Agent()

	// NewTestAgent setup a CA already by default

	// Register a local agent service
	reg := &api.AgentServiceRegistration{
		Name: "web",
		Port: 8080,
	}
	err := agent.ServiceRegister(reg)
	require.NoError(t, err)

	// Now we should be able to create a service that will eventually get it's TLS
	// all by itself!
	service, err := NewService("web", client)
	require.NoError(t, err)

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
	require.NotNil(t, tlsCfg.GetCertificate)
	leaf, err := tlsCfg.GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(leaf.Certificate[0])
	require.NoError(t, err)
	require.Len(t, cert.URIs, 1)
	require.True(t, strings.HasSuffix(cert.URIs[0].String(), "/svc/web"))

	// Verify it as a client would
	err = clientSideVerifier(tlsCfg, leaf.Certificate)
	require.NoError(t, err)

	// Now test that rotating the root updates
	{
		// Setup a new generated CA
		connect.TestCAConfigSet(t, a, nil)
	}

	// After some time, both root and leaves should be different but both should
	// still be correct.
	oldRootSubjects := getSubjects(tlsCfg.RootCAs)
	oldLeafSerial := cert.SerialNumber
	oldLeafKeyID := cert.SubjectKeyId
	retry.Run(t, func(r *retry.R) {
		updatedCfg := service.ServerTLSConfig()

		// Wait until roots are different
		rootSubjects := getSubjects(updatedCfg.RootCAs)
		if oldRootSubjects == rootSubjects {
			r.Fatalf("root certificates should have changed, got %s",
				rootSubjects)
		}

		leaf, err := updatedCfg.GetCertificate(&tls.ClientHelloInfo{})
		r.Check(err)
		cert, err := x509.ParseCertificate(leaf.Certificate[0])
		r.Check(err)

		if oldLeafSerial.Cmp(cert.SerialNumber) == 0 {
			r.Fatalf("leaf certificate should have changed, got serial %s",
				connect.EncodeSerialNumber(oldLeafSerial))
		}
		if bytes.Equal(oldLeafKeyID, cert.SubjectKeyId) {
			r.Fatalf("leaf should have a different key, got matching SubjectKeyID = %s",
				connect.HexString(oldLeafKeyID))
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
			//require.Equal(t,"https://backend.service.consul:443", addr)
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

		bodyBytes, err := io.ReadAll(resp.Body)
		r.Check(err)

		got := string(bodyBytes)
		want := "Hello, I am Backend"
		if got != want {
			r.Fatalf("got %s, want %s", got, want)
		}
	})
}

func TestService_HasDefaultHTTPResolverFromAddr(t *testing.T) {

	client, err := api.NewClient(api.DefaultConfig())
	require.NoError(t, err)

	s, err := NewService("foo", client)
	require.NoError(t, err)

	// Sanity check this is actually set in constructor since we always override
	// it in tests. Full tests of the resolver func are in resolver_test.go
	require.NotNil(t, s.httpResolverFromAddr)

	fn := s.httpResolverFromAddr

	expected := &ConsulResolver{
		Client:    client,
		Namespace: "default",
		Name:      "foo",
		Type:      ConsulResolverTypeService,
	}
	got, err := fn("foo.service.consul")
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func getSubjects(cp *x509.CertPool) string {
	subjectsIter := reflect.ValueOf(cp).Elem().FieldByName("byName").MapRange()
	subjects := []string{}
	for subjectsIter.Next() {
		k := subjectsIter.Key()
		subjects = append(subjects, k.String())
	}
	sort.Strings(subjects)
	subjectList := strings.Join(subjects, ",")
	return subjectList
}
