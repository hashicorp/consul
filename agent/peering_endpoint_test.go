package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	gpeer "google.golang.org/grpc/peer"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

var validCA = `
-----BEGIN CERTIFICATE-----
MIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg
Q0EgNzAeFw0xODA1MjExNjMzMjhaFw0yODA1MTgxNjMzMjhaMBYxFDASBgNVBAMT
C0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAER0qlxjnRcMEr
iSGlH7G7dYU7lzBEmLUSMZkyBbClmyV8+e8WANemjn+PLnCr40If9cmpr7RnC9Qk
GTaLnLiF16OCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/
MGgGA1UdDgRhBF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1OTpjMjpmYTo0ZTo3
NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToyNDpiMDowNDpiMzpl
ODo5Nzo1Yjo3ZTBqBgNVHSMEYzBhgF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1
OTpjMjpmYTo0ZTo3NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToy
NDpiMDowNDpiMzplODo5Nzo1Yjo3ZTA/BgNVHREEODA2hjRzcGlmZmU6Ly8xMjRk
ZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIuY29uc3VsMD0GA1UdHgEB
/wQzMDGgLzAtgisxMjRkZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIu
Y29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIQDzkkI7R+0U12a+zq2EQhP/n2mHmta+
fs2hBxWIELGwTAIgLdO7RRw+z9nnxCIA6kNl//mIQb+PGItespiHZKAz74Q=
-----END CERTIFICATE-----
`

func TestHTTP_Peering_Integration(t *testing.T) {
	// This is a full-stack integration test of the gRPC (internal) stack. We
	// use peering CRUD b/c that is one of the few endpoints exposed over gRPC
	// (internal).

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// We advertise a wan address we are not using, so that incidental attempts
	// to use it will loudly fail.
	const ip = "192.0.2.2"

	connectivityConfig := `
ports { serf_wan = -1 }
bind_addr = "0.0.0.0"
client_addr = "0.0.0.0"
advertise_addr = "127.0.0.1"
advertise_addr_wan = "` + ip + `" `

	var (
		buf1, buf2, buf3 bytes.Buffer
		testLog          = testutil.NewLogBuffer(t)

		log1 = io.MultiWriter(testLog, &buf1)
		log2 = io.MultiWriter(testLog, &buf2)
		log3 = io.MultiWriter(testLog, &buf3)
	)

	a1 := StartTestAgent(t, TestAgent{LogOutput: log1, HCL: `
		server = true
		bootstrap = false
		bootstrap_expect = 3
	` + connectivityConfig})
	t.Cleanup(func() { a1.Shutdown() })

	a2 := StartTestAgent(t, TestAgent{LogOutput: log2, HCL: `
		server = true
		bootstrap = false
		bootstrap_expect = 3
	` + connectivityConfig})
	t.Cleanup(func() { a2.Shutdown() })

	a3 := StartTestAgent(t, TestAgent{LogOutput: log3, HCL: `
		server = true
		bootstrap = false
		bootstrap_expect = 3
	` + connectivityConfig})
	t.Cleanup(func() { a3.Shutdown() })

	{ // join a2 to a1
		addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
		_, err := a1.JoinLAN([]string{addr}, nil)
		require.NoError(t, err)
	}
	{ // join a3 to a1
		addr := fmt.Sprintf("127.0.0.1:%d", a3.Config.SerfPortLAN)
		_, err := a1.JoinLAN([]string{addr}, nil)
		require.NoError(t, err)
	}

	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a1.RPC, "dc1", nil)

	testrpc.WaitForTestAgent(t, a1.RPC, "dc1")
	testrpc.WaitForTestAgent(t, a2.RPC, "dc1")
	testrpc.WaitForTestAgent(t, a3.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.LANMembersInAgentPartition(), 3)
		require.Len(r, a2.LANMembersInAgentPartition(), 3)
		require.Len(r, a3.LANMembersInAgentPartition(), 3)
	})

	type testcase struct {
		agent     *TestAgent
		peerName  string
		prevCount int
	}

	checkPeeringList := func(t *testing.T, a *TestAgent, expect int) {
		req, err := http.NewRequest("GET", "/v1/peerings", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		var apiResp []*api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))

		require.Len(t, apiResp, expect)
	}

	testConn := func(t *testing.T, conn *grpc.ClientConn, peers map[string]int) {
		rpcClientPeering := pbpeering.NewPeeringServiceClient(conn)

		peer := &gpeer.Peer{}
		_, err := rpcClientPeering.PeeringList(
			context.Background(),
			&pbpeering.PeeringListRequest{},
			grpc.Peer(peer),
		)
		require.NoError(t, err)

		peers[peer.Addr.String()]++
	}

	var (
		standardPeers = make(map[string]int)
		leaderPeers   = make(map[string]int)
	)
	runOnce := func(t *testing.T, tc testcase) {
		testutil.RunStep(t, "standard peers", func(t *testing.T) {
			conn, err := tc.agent.baseDeps.GRPCConnPool.ClientConn("dc1")
			require.NoError(t, err)
			testConn(t, conn, standardPeers)
		})

		testutil.RunStep(t, "leader peers", func(t *testing.T) {
			leaderConn, err := tc.agent.baseDeps.GRPCConnPool.ClientConnLeader()
			require.NoError(t, err)
			testConn(t, leaderConn, leaderPeers)
		})

		testutil.RunStep(t, "check peering list before", func(t *testing.T) {
			checkPeeringList(t, tc.agent, tc.prevCount)
		})

		body := &pbpeering.GenerateTokenRequest{
			PeerName: tc.peerName,
		}

		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		tc.agent.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var out pbpeering.GenerateTokenResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))

		testutil.RunStep(t, "check peering list after", func(t *testing.T) {
			checkPeeringList(t, tc.agent, tc.prevCount+1)
		})
	}

	// Try the procedure on all agents to force N-1 of them to leader-forward.
	cases := []testcase{
		{agent: a1, peerName: "peer-1", prevCount: 0},
		{agent: a2, peerName: "peer-2", prevCount: 1},
		{agent: a3, peerName: "peer-3", prevCount: 2},
	}

	for i, tc := range cases {
		tc := tc
		testutil.RunStep(t, "server-"+strconv.Itoa(i+1), func(t *testing.T) {
			runOnce(t, tc)
		})
	}

	testutil.RunStep(t, "ensure we got the right mixture of responses", func(t *testing.T) {
		// Disabling this assertion on the 1.13 backport
		// assert.Len(t, standardPeers, 3)

		// Each server talks to a single leader.
		assert.Len(t, leaderPeers, 1)
		for p, n := range leaderPeers {
			assert.Equal(t, 3, n, "leader peer %q expected 3 uses", p)
		}
	})

	testutil.RunStep(t, "no server experienced the server resolution error", func(t *testing.T) {
		// Check them all for the bad error
		const grpcError = `failed to find Consul server for global address`

		var buf bytes.Buffer
		buf.ReadFrom(&buf1)
		buf.ReadFrom(&buf2)
		buf.ReadFrom(&buf3)

		scan := bufio.NewScanner(&buf)
		for scan.Scan() {
			line := scan.Text()
			require.NotContains(t, line, grpcError)
		}
		require.NoError(t, scan.Err())
	})
}

func TestHTTP_Peering_GenerateToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("No Body", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/token", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "The peering arguments must be provided in the body")
	})

	t.Run("Body Invalid", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader([]byte("abc")))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "Body decoding failed:")
	})

	t.Run("No Name", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/token",
			bytes.NewReader([]byte(`{}`)))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "PeerName is required")
	})

	// TODO(peering): add more failure cases

	t.Run("Success", func(t *testing.T) {
		body := &pbpeering.GenerateTokenRequest{
			PeerName: "peering-a",
		}

		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var r pbpeering.GenerateTokenResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&r))

		tokenJSON, err := base64.StdEncoding.DecodeString(r.PeeringToken)
		require.NoError(t, err)

		var token structs.PeeringToken
		require.NoError(t, json.Unmarshal(tokenJSON, &token))

		require.Nil(t, token.CA)
		require.Equal(t, []string{fmt.Sprintf("127.0.0.1:%d", a.config.GRPCPort)}, token.ServerAddresses)
		require.Equal(t, "server.dc1.consul", token.ServerName)

		// The PeerID in the token is randomly generated so we don't assert on its value.
		require.NotEmpty(t, token.PeerID)
	})

	t.Run("Success with external address", func(t *testing.T) {
		externalAddress := "32.1.2.3"
		body := &pbpeering.GenerateTokenRequest{
			PeerName:                "peering-a",
			ServerExternalAddresses: []string{externalAddress},
		}

		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var r pbpeering.GenerateTokenResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&r))

		tokenJSON, err := base64.StdEncoding.DecodeString(r.PeeringToken)
		require.NoError(t, err)

		var token structs.PeeringToken
		require.NoError(t, json.Unmarshal(tokenJSON, &token))

		require.Nil(t, token.CA)
		require.Equal(t, []string{externalAddress}, token.ServerAddresses)
		require.Equal(t, "server.dc1.consul", token.ServerName)

		// The PeerID in the token is randomly generated so we don't assert on its value.
		require.NotEmpty(t, token.PeerID)
	})
}

// Test for GenerateToken calls at various points in a peer's lifecycle
func TestHTTP_Peering_GenerateToken_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	body := &pbpeering.GenerateTokenRequest{
		PeerName: "peering-a",
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	getPeering := func(t *testing.T) *api.Peering {
		t.Helper()
		// Check state of peering
		req, err := http.NewRequest("GET", "/v1/peering/peering-a", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var p *api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&p))
		return p
	}

	{
		// Call once
		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())
		// Assertions tested in TestHTTP_Peering_GenerateToken
	}

	if !t.Run("generate token called again", func(t *testing.T) {
		before := getPeering(t)
		require.Equal(t, api.PeeringStatePending, before.State)

		// Call again
		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		after := getPeering(t)
		assert.NotEqual(t, before.ModifyIndex, after.ModifyIndex)
		// blank out modify index so we can compare rest of struct
		before.ModifyIndex, after.ModifyIndex = 0, 0
		assert.Equal(t, before, after)

	}) {
		t.FailNow()
	}

}

func TestHTTP_Peering_Establish(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("No Body", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/establish", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "The peering arguments must be provided in the body")
	})

	t.Run("Body Invalid", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/establish", bytes.NewReader([]byte("abc")))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "Body decoding failed:")
	})

	t.Run("No Name", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/establish",
			bytes.NewReader([]byte(`{}`)))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "PeerName is required")
	})

	t.Run("No Token", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/establish",
			bytes.NewReader([]byte(`{"PeerName": "peer1-usw1"}`)))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "PeeringToken is required")
	})

	t.Run("Success", func(t *testing.T) {
		a2 := NewTestAgent(t, `datacenter = "dc2"`)
		testrpc.WaitForTestAgent(t, a2.RPC, "dc2")

		bodyBytes, err := json.Marshal(&pbpeering.GenerateTokenRequest{
			PeerName: "foo",
		})
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var r pbpeering.GenerateTokenResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&r))

		b, err := json.Marshal(&pbpeering.EstablishRequest{
			PeerName:     "zip",
			PeeringToken: r.PeeringToken,
			Meta:         map[string]string{"foo": "bar"},
		})
		require.NoError(t, err)

		req, err = http.NewRequest("POST", "/v1/peering/establish", bytes.NewReader(b))
		require.NoError(t, err)
		resp = httptest.NewRecorder()
		a2.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		// success response does not currently return a value so {} is correct
		require.Equal(t, "{}", resp.Body.String())
	})
}

func TestHTTP_Peering_MethodNotAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	foo := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "foo",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerCAPems:          nil,
			PeerServerName:      "fooservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err := a.rpcClientPeering.PeeringWrite(ctx, foo)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", "/v1/peering/foo", nil)
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusMethodNotAllowed, resp.Code)
}

func TestHTTP_Peering_Read(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	foo := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "foo",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerCAPems:          nil,
			PeerServerName:      "fooservername",
			PeerServerAddresses: []string{"addr1"},
			Meta:                map[string]string{"foo": "bar"},
		},
	}
	_, err := a.rpcClientPeering.PeeringWrite(ctx, foo)
	require.NoError(t, err)
	bar := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "bar",
			State:               pbpeering.PeeringState_ACTIVE,
			PeerCAPems:          nil,
			PeerServerName:      "barservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err = a.rpcClientPeering.PeeringWrite(ctx, bar)
	require.NoError(t, err)

	t.Run("return foo", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peering/foo", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		var apiResp api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))

		require.Equal(t, foo.Peering.Name, apiResp.Name)
		require.Equal(t, foo.Peering.Meta, apiResp.Meta)

		require.Equal(t, uint64(0), apiResp.ImportedServiceCount)
		require.Equal(t, uint64(0), apiResp.ExportedServiceCount)

	})

	t.Run("not found", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peering/baz", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Code)
		require.Equal(t, "Peering not found for \"baz\"", resp.Body.String())
	})
}

func TestHTTP_Peering_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	bodyBytes, err := json.Marshal(&pbpeering.GenerateTokenRequest{
		PeerName: "foo",
	})
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

	t.Run("read existing token before attempting delete", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peering/foo", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		var apiResp api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))
		require.Equal(t, "foo", apiResp.Name)
	})

	t.Run("delete the existing token we just read", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", "/v1/peering/foo", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Equal(t, "", resp.Body.String())
	})

	t.Run("now the token is deleted and reads should yield a 404", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, err := http.NewRequest("GET", "/v1/peering/foo", nil)
			require.NoError(r, err)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusNotFound, resp.Code)
		})
	})

	t.Run("delete a token that does not exist", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", "/v1/peering/baz", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestHTTP_Peering_List(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	foo := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "foo",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerCAPems:          nil,
			PeerServerName:      "fooservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err := a.rpcClientPeering.PeeringWrite(ctx, foo)
	require.NoError(t, err)
	bar := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "bar",
			State:               pbpeering.PeeringState_ACTIVE,
			PeerCAPems:          nil,
			PeerServerName:      "barservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err = a.rpcClientPeering.PeeringWrite(ctx, bar)
	require.NoError(t, err)

	t.Run("return all", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peerings", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		var apiResp []*api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))

		require.Len(t, apiResp, 2)

		for _, p := range apiResp {
			require.Equal(t, uint64(0), p.ImportedServiceCount)
			require.Equal(t, uint64(0), p.ExportedServiceCount)
		}
	})
}
