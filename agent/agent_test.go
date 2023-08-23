package agent

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	mathrand "math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/tcpproxy"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

func getService(a *TestAgent, id string) *structs.NodeService {
	return a.State.Service(structs.NewServiceID(id, nil))
}

func getCheck(a *TestAgent, id types.CheckID) *structs.HealthCheck {
	return a.State.Check(structs.NewCheckID(id, nil))
}

func requireServiceExists(t *testing.T, a *TestAgent, id string) *structs.NodeService {
	t.Helper()
	svc := getService(a, id)
	require.NotNil(t, svc, "missing service %q", id)
	return svc
}

func requireServiceMissing(t *testing.T, a *TestAgent, id string) {
	t.Helper()
	require.Nil(t, getService(a, id), "have service %q (expected missing)", id)
}

func requireCheckExists(t *testing.T, a *TestAgent, id types.CheckID) *structs.HealthCheck {
	t.Helper()
	chk := getCheck(a, id)
	require.NotNil(t, chk, "missing check %q", id)
	return chk
}

func requireCheckMissing(t *testing.T, a *TestAgent, id types.CheckID) {
	t.Helper()
	require.Nil(t, getCheck(a, id), "have check %q (expected missing)", id)
}

func requireCheckExistsMap(t *testing.T, m interface{}, id types.CheckID) {
	t.Helper()
	require.Contains(t, m, structs.NewCheckID(id, nil), "missing check %q", id)
}

func requireCheckMissingMap(t *testing.T, m interface{}, id types.CheckID) {
	t.Helper()
	require.NotContains(t, m, structs.NewCheckID(id, nil), "have check %q (expected missing)", id)
}

func TestAgent_MultiStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for i := 0; i < 10; i++ {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			a := NewTestAgent(t, "")
			time.Sleep(250 * time.Millisecond)
			a.Shutdown()
		})
	}
}

func TestAgent_ConnectClusterIDConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		name          string
		hcl           string
		wantClusterID string
		wantErr       bool
	}{
		{
			name:          "default TestAgent has fixed cluster id",
			hcl:           "",
			wantClusterID: connect.TestClusterID,
		},
		{
			name:          "no cluster ID specified sets to test ID",
			hcl:           "connect { enabled = true }",
			wantClusterID: connect.TestClusterID,
		},
		{
			name: "non-UUID cluster_id is fatal",
			hcl: `connect {
	   enabled = true
	   ca_config {
	     cluster_id = "fake-id"
	   }
	 }`,
			wantClusterID: "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := TestAgent{HCL: tt.hcl}
			err := a.Start(t)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return // don't run the rest of the test
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
			defer a.Shutdown()

			cfg := a.consulConfig()
			assert.Equal(t, tt.wantClusterID, cfg.CAConfig.ClusterID)
		})
	}
}

func TestAgent_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	if err := a.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := a.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case <-a.ShutdownCh():
	default:
		t.Fatalf("should be closed")
	}
}

func TestAgent_RPCPing(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	var out struct{}
	if err := a.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_TokenStore(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, `
		acl {
			tokens {
				default = "user"
				agent = "agent"
				agent_recovery = "recovery"
			}
		}
	`)
	defer a.Shutdown()

	if got, want := a.tokens.UserToken(), "user"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := a.tokens.AgentToken(), "agent"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := a.tokens.IsAgentRecoveryToken("recovery"), true; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAgent_ReconnectConfigSettings(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	func() {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		lan := a.consulConfig().SerfLANConfig.ReconnectTimeout
		if lan != 3*24*time.Hour {
			t.Fatalf("bad: %s", lan.String())
		}

		wan := a.consulConfig().SerfWANConfig.ReconnectTimeout
		if wan != 3*24*time.Hour {
			t.Fatalf("bad: %s", wan.String())
		}
	}()

	func() {
		a := NewTestAgent(t, `
			reconnect_timeout = "24h"
			reconnect_timeout_wan = "36h"
		`)
		defer a.Shutdown()

		lan := a.consulConfig().SerfLANConfig.ReconnectTimeout
		if lan != 24*time.Hour {
			t.Fatalf("bad: %s", lan.String())
		}

		wan := a.consulConfig().SerfWANConfig.ReconnectTimeout
		if wan != 36*time.Hour {
			t.Fatalf("bad: %s", wan.String())
		}
	}()
}

func TestAgent_HTTPMaxHeaderBytes(t *testing.T) {
	tests := []struct {
		name                 string
		maxHeaderBytes       int
		expectedHTTPResponse int
	}{
		{
			"max header bytes 1 returns 431 http response when too large headers are sent",
			1,
			431,
		},
		{
			"max header bytes 0 returns 200 http response, as the http.DefaultMaxHeaderBytes size of 1MB is used",
			0,
			200,
		},
		{
			"negative maxHeaderBytes returns 200 http response, as the http.DefaultMaxHeaderBytes size of 1MB is used",
			-10,
			200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caConfig := tlsutil.Config{}
			tlsConf, err := tlsutil.NewConfigurator(caConfig, hclog.New(nil))
			require.NoError(t, err)

			bd := BaseDeps{
				Deps: consul.Deps{
					Logger:          hclog.NewInterceptLogger(nil),
					Tokens:          new(token.Store),
					TLSConfigurator: tlsConf,
					GRPCConnPool:    &fakeGRPCConnPool{},
				},
				RuntimeConfig: &config.RuntimeConfig{
					HTTPAddrs: []net.Addr{
						&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: freeport.GetOne(t)},
					},
					HTTPMaxHeaderBytes: tt.maxHeaderBytes,
				},
				Cache: cache.New(cache.Options{}),
			}
			bd, err = initEnterpriseBaseDeps(bd, nil)
			require.NoError(t, err)

			a, err := New(bd)
			require.NoError(t, err)

			a.startLicenseManager(testutil.TestContext(t))

			srvs, err := a.listenHTTP()
			require.NoError(t, err)

			require.Equal(t, tt.maxHeaderBytes, a.config.HTTPMaxHeaderBytes)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			t.Cleanup(cancel)

			g := new(errgroup.Group)
			for _, s := range srvs {
				g.Go(s.Run)
			}

			require.Len(t, srvs, 1)

			client := &http.Client{}
			for _, s := range srvs {
				u := url.URL{Scheme: s.Protocol, Host: s.Addr.String()}
				req, err := http.NewRequest(http.MethodGet, u.String(), nil)
				require.NoError(t, err)

				// This is directly pulled from the testing of request limits in the net/http source
				// https://github.com/golang/go/blob/go1.15.3/src/net/http/serve_test.go#L2897-L2900
				var bytesPerHeader = len("header12345: val12345\r\n")
				for i := 0; i < ((tt.maxHeaderBytes+4096)/bytesPerHeader)+1; i++ {
					req.Header.Set(fmt.Sprintf("header%05d", i), fmt.Sprintf("val%05d", i))
				}

				resp, err := client.Do(req.WithContext(ctx))
				require.NoError(t, err)
				require.Equal(t, tt.expectedHTTPResponse, resp.StatusCode, "expected a '%d' http response, got '%d'", tt.expectedHTTPResponse, resp.StatusCode)
			}
		})
	}
}

type fakeGRPCConnPool struct{}

func (f fakeGRPCConnPool) ClientConn(_ string) (*grpc.ClientConn, error) {
	return nil, nil
}

func (f fakeGRPCConnPool) ClientConnLeader() (*grpc.ClientConn, error) {
	return nil, nil
}

func (f fakeGRPCConnPool) SetGatewayResolver(_ func(string) string) {
}

func TestAgent_ReconnectConfigWanDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, `
		ports { serf_wan = -1 }
		reconnect_timeout_wan = "36h"
	`)
	defer a.Shutdown()

	// This is also testing that we dont panic like before #4515
	require.Nil(t, a.consulConfig().SerfWANConfig)
}

func TestAgent_AddService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_AddService(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_AddService(t, "enable_central_service_config = true")
	})
}

func testAgent_AddService(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		node_name = "node1"
	`+extraHCL)
	defer a.Shutdown()

	tests := []struct {
		desc       string
		srv        *structs.NodeService
		wantSrv    func(ns *structs.NodeService)
		chkTypes   []*structs.CheckType
		healthChks map[string]*structs.HealthCheck
	}{
		{
			"one check",
			&structs.NodeService{
				ID:             "svcid1",
				Service:        "svcname1",
				Tags:           []string{"tag1"},
				Weights:        nil, // nil weights...
				Port:           8100,
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			// ... should be populated to avoid "IsSame" returning true during AE.
			func(ns *structs.NodeService) {
				ns.Weights = &structs.Weights{
					Passing: 1,
					Warning: 1,
				}
			},
			[]*structs.CheckType{
				{
					CheckID: "check1",
					Name:    "name1",
					TTL:     time.Minute,
					Notes:   "note1",
				},
			},
			map[string]*structs.HealthCheck{
				"check1": {
					Node:           "node1",
					CheckID:        "check1",
					Name:           "name1",
					Interval:       "",
					Timeout:        "", // these are empty because a TTL was provided
					Status:         "critical",
					Notes:          "note1",
					ServiceID:      "svcid1",
					ServiceName:    "svcname1",
					ServiceTags:    []string{"tag1"},
					Type:           "ttl",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		},
		{
			"multiple checks",
			&structs.NodeService{
				ID:      "svcid2",
				Service: "svcname2",
				Weights: &structs.Weights{
					Passing: 2,
					Warning: 1,
				},
				Tags:           []string{"tag2"},
				Port:           8200,
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			nil, // No change expected
			[]*structs.CheckType{
				{
					CheckID: "check1",
					Name:    "name1",
					TTL:     time.Minute,
					Notes:   "note1",
				},
				{
					CheckID: "check-noname",
					TTL:     time.Minute,
				},
				{
					Name: "check-noid",
					TTL:  time.Minute,
				},
				{
					TTL: time.Minute,
				},
			},
			map[string]*structs.HealthCheck{
				"check1": {
					Node:           "node1",
					CheckID:        "check1",
					Name:           "name1",
					Interval:       "",
					Timeout:        "", // these are empty bcause a TTL was provided
					Status:         "critical",
					Notes:          "note1",
					ServiceID:      "svcid2",
					ServiceName:    "svcname2",
					ServiceTags:    []string{"tag2"},
					Type:           "ttl",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				"check-noname": {
					Node:           "node1",
					CheckID:        "check-noname",
					Name:           "Service 'svcname2' check",
					Interval:       "",
					Timeout:        "", // these are empty because a TTL was provided
					Status:         "critical",
					ServiceID:      "svcid2",
					ServiceName:    "svcname2",
					ServiceTags:    []string{"tag2"},
					Type:           "ttl",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				"service:svcid2:3": {
					Node:           "node1",
					CheckID:        "service:svcid2:3",
					Name:           "check-noid",
					Interval:       "",
					Timeout:        "", // these are empty becuase a TTL was provided
					Status:         "critical",
					ServiceID:      "svcid2",
					ServiceName:    "svcname2",
					ServiceTags:    []string{"tag2"},
					Type:           "ttl",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				"service:svcid2:4": {
					Node:           "node1",
					CheckID:        "service:svcid2:4",
					Name:           "Service 'svcname2' check",
					Interval:       "",
					Timeout:        "", // these are empty because a TTL was provided
					Status:         "critical",
					ServiceID:      "svcid2",
					ServiceName:    "svcname2",
					ServiceTags:    []string{"tag2"},
					Type:           "ttl",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// check the service registration
			t.Run(tt.srv.ID, func(t *testing.T) {
				err := a.addServiceFromSource(tt.srv, tt.chkTypes, false, "", ConfigSourceLocal)
				if err != nil {
					t.Fatalf("err: %v", err)
				}

				got := getService(a, tt.srv.ID)
				// Make a copy since the tt.srv points to the one in memory in the local
				// state still so changing it is a tautology!
				want := *tt.srv
				if tt.wantSrv != nil {
					tt.wantSrv(&want)
				}
				require.Equal(t, &want, got)
				require.True(t, got.IsSame(&want))
			})

			// check the health checks
			for k, v := range tt.healthChks {
				t.Run(k, func(t *testing.T) {
					got := getCheck(a, types.CheckID(k))
					require.Equal(t, v, got)
				})
			}

			// check the ttl checks
			for k := range tt.healthChks {
				t.Run(k+" ttl", func(t *testing.T) {
					chk := a.checkTTLs[structs.NewCheckID(types.CheckID(k), nil)]
					if chk == nil {
						t.Fatal("got nil want TTL check")
					}
					if got, want := string(chk.CheckID.ID), k; got != want {
						t.Fatalf("got CheckID %v want %v", got, want)
					}
					if got, want := chk.TTL, time.Minute; got != want {
						t.Fatalf("got TTL %v want %v", got, want)
					}
				})
			}
		})
	}
}

// addServiceFromSource is a test helper that exists to maintain an old function
// signature that was used in many tests.
// Deprecated: use AddService
func (a *Agent) addServiceFromSource(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, source configSource) error {
	return a.AddService(AddServiceRequest{
		Service:               service,
		chkTypes:              chkTypes,
		persist:               persist,
		token:                 token,
		replaceExistingChecks: false,
		Source:                source,
	})
}

func TestAgent_AddServices_AliasUpdateCheckNotReverted(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_AddServices_AliasUpdateCheckNotReverted(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_AddServices_AliasUpdateCheckNotReverted(t, "enable_central_service_config = true")
	})
}

func testAgent_AddServices_AliasUpdateCheckNotReverted(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		node_name = "node1"
	`+extraHCL)
	defer a.Shutdown()

	// It's tricky to get an UpdateCheck call to be timed properly so it lands
	// right in the middle of an addServiceInternal call so we cheat a bit and
	// rely upon alias checks to do that work for us.  We add enough services
	// that probabilistically one of them is going to end up properly in the
	// critical section.
	//
	// The first number I picked here (10) surprisingly failed every time prior
	// to PR #6144 solving the underlying problem.
	const numServices = 10

	services := make([]*structs.ServiceDefinition, numServices)
	checkIDs := make([]types.CheckID, numServices)
	services[0] = &structs.ServiceDefinition{
		ID:     "fake",
		Name:   "fake",
		Port:   8080,
		Checks: []*structs.CheckType{},
	}
	for i := 1; i < numServices; i++ {
		name := fmt.Sprintf("web-%d", i)

		services[i] = &structs.ServiceDefinition{
			ID:   name,
			Name: name,
			Port: 8080 + i,
			Checks: []*structs.CheckType{
				{
					Name:         "alias-for-fake-service",
					AliasService: "fake",
				},
			},
		}

		checkIDs[i] = types.CheckID("service:" + name)
	}

	// Add all of the services quickly as you might do from config file snippets.
	for _, service := range services {
		ns := service.NodeService()

		chkTypes, err := service.CheckTypes()
		require.NoError(t, err)

		require.NoError(t, a.addServiceFromSource(ns, chkTypes, false, service.Token, ConfigSourceLocal))
	}

	retry.Run(t, func(r *retry.R) {
		gotChecks := a.State.Checks(nil)
		for id, check := range gotChecks {
			require.Equal(r, "passing", check.Status, "check %q is wrong", id)
			require.Equal(r, "No checks found.", check.Output, "check %q is wrong", id)
		}
	})
}

func test_createAlias(t *testing.T, agent *TestAgent, chk *structs.CheckType, expectedResult string) func(r *retry.R) {
	t.Helper()
	serviceNum := mathrand.Int()
	srv := &structs.NodeService{
		Service: fmt.Sprintf("serviceAlias-%d", serviceNum),
		Tags:    []string{"tag1"},
		Port:    8900 + serviceNum,
	}
	if srv.ID == "" {
		srv.ID = fmt.Sprintf("serviceAlias-%d", serviceNum)
	}
	chk.Status = api.HealthWarning
	if chk.CheckID == "" {
		chk.CheckID = types.CheckID(fmt.Sprintf("check-%d", serviceNum))
	}
	err := agent.addServiceFromSource(srv, []*structs.CheckType{chk}, false, "", ConfigSourceLocal)
	assert.NoError(t, err)
	return func(r *retry.R) {
		t.Helper()
		found := false
		for _, c := range agent.State.CheckStates(structs.WildcardEnterpriseMetaInDefaultPartition()) {
			if c.Check.CheckID == chk.CheckID {
				found = true
				assert.Equal(t, expectedResult, c.Check.Status, "Check state should be %s, was %s in %#v", expectedResult, c.Check.Status, c.Check)
				srvID := structs.NewServiceID(srv.ID, structs.WildcardEnterpriseMetaInDefaultPartition())
				if err := agent.Agent.State.RemoveService(srvID); err != nil {
					fmt.Println("[DEBUG] Fail to remove service", srvID, ", err:=", err)
				}
				fmt.Println("[DEBUG] Service Removed", srvID, ", err:=", err)
				break
			}
		}
		assert.True(t, found)
	}
}

// TestAgent_CheckAliasRPC test the Alias Check to be properly sync remotely
// and locally.
// It contains a few hacks such as unlockIndexOnNode because watch performed
// in CheckAlias.runQuery() waits for 1 min, so Shutdoww the agent might take time
// So, we ensure the agent will update regularilly the index
func TestAgent_CheckAliasRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Helper()

	a := NewTestAgent(t, `
		node_name = "node1"
	`)

	srv := &structs.NodeService{
		ID:      "svcid1",
		Service: "svcname1",
		Tags:    []string{"tag1"},
		Port:    8100,
	}
	unlockIndexOnNode := func() {
		// We ensure to not block and update Agent's index
		srv.Tags = []string{fmt.Sprintf("tag-%s", time.Now())}
		assert.NoError(t, a.waitForUp())
		err := a.addServiceFromSource(srv, []*structs.CheckType{}, false, "", ConfigSourceLocal)
		assert.NoError(t, err)
	}
	shutdownAgent := func() {
		// This is to be sure Alias Checks on remote won't be blocked during 1 min
		unlockIndexOnNode()
		fmt.Println("[DEBUG] STARTING shutdown for TestAgent_CheckAliasRPC", time.Now())
		go a.Shutdown()
		unlockIndexOnNode()
		fmt.Println("[DEBUG] DONE shutdown for TestAgent_CheckAliasRPC", time.Now())
	}
	defer shutdownAgent()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	assert.NoError(t, a.waitForUp())
	err := a.addServiceFromSource(srv, []*structs.CheckType{}, false, "", ConfigSourceLocal)
	assert.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		t.Helper()
		var args structs.NodeSpecificRequest
		args.Datacenter = "dc1"
		args.Node = "node1"
		args.AllowStale = true
		var out structs.IndexedNodeServices
		err := a.RPC("Catalog.NodeServices", &args, &out)
		assert.NoError(r, err)
		foundService := false
		lookup := structs.NewServiceID("svcid1", structs.WildcardEnterpriseMetaInDefaultPartition())
		for _, srv := range out.NodeServices.Services {
			if lookup.Matches(srv.CompoundServiceID()) {
				foundService = true
			}
		}
		assert.True(r, foundService, "could not find svcid1 in %#v", out.NodeServices.Services)
	})

	checks := make([](func(*retry.R)), 0)

	checks = append(checks, test_createAlias(t, a, &structs.CheckType{
		Name:         "Check_Local_Ok",
		AliasService: "svcid1",
	}, api.HealthPassing))

	checks = append(checks, test_createAlias(t, a, &structs.CheckType{
		Name:         "Check_Local_Fail",
		AliasService: "svcidNoExistingID",
	}, api.HealthCritical))

	checks = append(checks, test_createAlias(t, a, &structs.CheckType{
		Name:         "Check_Remote_Host_Ok",
		AliasNode:    "node1",
		AliasService: "svcid1",
	}, api.HealthPassing))

	checks = append(checks, test_createAlias(t, a, &structs.CheckType{
		Name:         "Check_Remote_Host_Non_Existing_Service",
		AliasNode:    "node1",
		AliasService: "svcidNoExistingID",
	}, api.HealthCritical))

	// We wait for max 5s for all checks to be in sync
	{
		for i := 0; i < 50; i++ {
			unlockIndexOnNode()
			allNonWarning := true
			for _, chk := range a.State.Checks(structs.WildcardEnterpriseMetaInDefaultPartition()) {
				if chk.Status == api.HealthWarning {
					allNonWarning = false
				}
			}
			if allNonWarning {
				break
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	for _, toRun := range checks {
		unlockIndexOnNode()
		retry.Run(t, toRun)
	}
}

func TestAgent_AddServiceWithH2PINGCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	check := []*structs.CheckType{
		{
			CheckID:       "test-h2ping-check",
			Name:          "test-h2ping-check",
			H2PING:        "localhost:12345",
			TLSSkipVerify: true,
			Interval:      10 * time.Second,
		},
	}

	nodeService := &structs.NodeService{
		ID:      "test-h2ping-check-service",
		Service: "test-h2ping-check-service",
	}
	err := a.addServiceFromSource(nodeService, check, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("Error registering service: %v", err)
	}
	requireCheckExists(t, a, "test-h2ping-check")
}

func TestAgent_AddServiceWithH2CPINGCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	check := []*structs.CheckType{
		{
			CheckID:       "test-h2cping-check",
			Name:          "test-h2cping-check",
			H2PING:        "localhost:12345",
			TLSSkipVerify: true,
			Interval:      10 * time.Second,
			H2PingUseTLS:  false,
		},
	}

	nodeService := &structs.NodeService{
		ID:      "test-h2cping-check-service",
		Service: "test-h2cping-check-service",
	}
	err := a.addServiceFromSource(nodeService, check, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("Error registering service: %v", err)
	}
	requireCheckExists(t, a, "test-h2cping-check")
}

func TestAgent_AddServiceNoExec(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_AddServiceNoExec(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_AddServiceNoExec(t, "enable_central_service_config = true")
	})
}

func testAgent_AddServiceNoExec(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		node_name = "node1"
	`+extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	srv := &structs.NodeService{
		ID:      "svcid1",
		Service: "svcname1",
		Tags:    []string{"tag1"},
		Port:    8100,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}

	err := a.addServiceFromSource(srv, []*structs.CheckType{chk}, false, "", ConfigSourceLocal)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}

	err = a.addServiceFromSource(srv, []*structs.CheckType{chk}, false, "", ConfigSourceRemote)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_AddServiceNoRemoteExec(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_AddServiceNoRemoteExec(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_AddServiceNoRemoteExec(t, "enable_central_service_config = true")
	})
}

func testAgent_AddServiceNoRemoteExec(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		node_name = "node1"
		enable_local_script_checks = true
	`+extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	srv := &structs.NodeService{
		ID:      "svcid1",
		Service: "svcname1",
		Tags:    []string{"tag1"},
		Port:    8100,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}

	err := a.addServiceFromSource(srv, []*structs.CheckType{chk}, false, "", ConfigSourceRemote)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}
}

func TestAddServiceIPv4TaggedDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Helper()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	srv := &structs.NodeService{
		Service: "my_service",
		ID:      "my_service_id",
		Port:    8100,
		Address: "10.0.1.2",
	}

	err := a.addServiceFromSource(srv, []*structs.CheckType{}, false, "", ConfigSourceRemote)
	require.Nil(t, err)

	ns := a.State.Service(structs.NewServiceID("my_service_id", nil))
	require.NotNil(t, ns)

	svcAddr := structs.ServiceAddress{Address: srv.Address, Port: srv.Port}
	require.Equal(t, svcAddr, ns.TaggedAddresses[structs.TaggedAddressLANIPv4])
	require.Equal(t, svcAddr, ns.TaggedAddresses[structs.TaggedAddressWANIPv4])
	_, ok := ns.TaggedAddresses[structs.TaggedAddressLANIPv6]
	require.False(t, ok)
	_, ok = ns.TaggedAddresses[structs.TaggedAddressWANIPv6]
	require.False(t, ok)
}

func TestAddServiceIPv6TaggedDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Helper()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	srv := &structs.NodeService{
		Service: "my_service",
		ID:      "my_service_id",
		Port:    8100,
		Address: "::5",
	}

	err := a.addServiceFromSource(srv, []*structs.CheckType{}, false, "", ConfigSourceRemote)
	require.Nil(t, err)

	ns := a.State.Service(structs.NewServiceID("my_service_id", nil))
	require.NotNil(t, ns)

	svcAddr := structs.ServiceAddress{Address: srv.Address, Port: srv.Port}
	require.Equal(t, svcAddr, ns.TaggedAddresses[structs.TaggedAddressLANIPv6])
	require.Equal(t, svcAddr, ns.TaggedAddresses[structs.TaggedAddressWANIPv6])
	_, ok := ns.TaggedAddresses[structs.TaggedAddressLANIPv4]
	require.False(t, ok)
	_, ok = ns.TaggedAddresses[structs.TaggedAddressWANIPv4]
	require.False(t, ok)
}

func TestAddServiceIPv4TaggedSet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Helper()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	srv := &structs.NodeService{
		Service: "my_service",
		ID:      "my_service_id",
		Port:    8100,
		Address: "10.0.1.2",
		TaggedAddresses: map[string]structs.ServiceAddress{
			structs.TaggedAddressWANIPv4: {
				Address: "10.100.200.5",
				Port:    8100,
			},
		},
	}

	err := a.addServiceFromSource(srv, []*structs.CheckType{}, false, "", ConfigSourceRemote)
	require.Nil(t, err)

	ns := a.State.Service(structs.NewServiceID("my_service_id", nil))
	require.NotNil(t, ns)

	svcAddr := structs.ServiceAddress{Address: srv.Address, Port: srv.Port}
	require.Equal(t, svcAddr, ns.TaggedAddresses[structs.TaggedAddressLANIPv4])
	require.Equal(t, structs.ServiceAddress{Address: "10.100.200.5", Port: 8100}, ns.TaggedAddresses[structs.TaggedAddressWANIPv4])
	_, ok := ns.TaggedAddresses[structs.TaggedAddressLANIPv6]
	require.False(t, ok)
	_, ok = ns.TaggedAddresses[structs.TaggedAddressWANIPv6]
	require.False(t, ok)
}

func TestAddServiceIPv6TaggedSet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Helper()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	srv := &structs.NodeService{
		Service: "my_service",
		ID:      "my_service_id",
		Port:    8100,
		Address: "::5",
		TaggedAddresses: map[string]structs.ServiceAddress{
			structs.TaggedAddressWANIPv6: {
				Address: "::6",
				Port:    8100,
			},
		},
	}

	err := a.addServiceFromSource(srv, []*structs.CheckType{}, false, "", ConfigSourceRemote)
	require.Nil(t, err)

	ns := a.State.Service(structs.NewServiceID("my_service_id", nil))
	require.NotNil(t, ns)

	svcAddr := structs.ServiceAddress{Address: srv.Address, Port: srv.Port}
	require.Equal(t, svcAddr, ns.TaggedAddresses[structs.TaggedAddressLANIPv6])
	require.Equal(t, structs.ServiceAddress{Address: "::6", Port: 8100}, ns.TaggedAddresses[structs.TaggedAddressWANIPv6])
	_, ok := ns.TaggedAddresses[structs.TaggedAddressLANIPv4]
	require.False(t, ok)
	_, ok = ns.TaggedAddresses[structs.TaggedAddressWANIPv4]
	require.False(t, ok)
}

func TestAgent_RemoveService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RemoveService(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RemoveService(t, "enable_central_service_config = true")
	})
}

func testAgent_RemoveService(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()

	// Remove a service that doesn't exist
	if err := a.RemoveService(structs.NewServiceID("redis", nil)); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove without an ID
	if err := a.RemoveService(structs.NewServiceID("", nil)); err == nil {
		t.Fatalf("should have errored")
	}

	// Removing a service with a single check works
	{
		srv := &structs.NodeService{
			ID:      "memcache",
			Service: "memcache",
			Port:    8000,
		}
		chkTypes := []*structs.CheckType{{TTL: time.Minute}}

		if err := a.addServiceFromSource(srv, chkTypes, false, "", ConfigSourceLocal); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Add a check after the fact with a specific check ID
		check := &structs.CheckDefinition{
			ID:        "check2",
			Name:      "check2",
			ServiceID: "memcache",
			TTL:       time.Minute,
		}
		hc := check.HealthCheck("node1")
		if err := a.AddCheck(hc, check.CheckType(), false, "", ConfigSourceLocal); err != nil {
			t.Fatalf("err: %s", err)
		}

		if err := a.RemoveService(structs.NewServiceID("memcache", nil)); err != nil {
			t.Fatalf("err: %s", err)
		}
		require.Nil(t, a.State.Check(structs.NewCheckID("service:memcache", nil)), "have memcache check")
		require.Nil(t, a.State.Check(structs.NewCheckID("check2", nil)), "have check2 check")
	}

	// Removing a service with multiple checks works
	{
		// add a service to remove
		srv := &structs.NodeService{
			ID:      "redis",
			Service: "redis",
			Port:    8000,
		}
		chkTypes := []*structs.CheckType{
			{TTL: time.Minute},
			{TTL: 30 * time.Second},
		}
		if err := a.addServiceFromSource(srv, chkTypes, false, "", ConfigSourceLocal); err != nil {
			t.Fatalf("err: %v", err)
		}

		// add another service that wont be affected
		srv = &structs.NodeService{
			ID:      "mysql",
			Service: "mysql",
			Port:    3306,
		}
		chkTypes = []*structs.CheckType{
			{TTL: time.Minute},
			{TTL: 30 * time.Second},
		}
		if err := a.addServiceFromSource(srv, chkTypes, false, "", ConfigSourceLocal); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Remove the service
		if err := a.RemoveService(structs.NewServiceID("redis", nil)); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Ensure we have a state mapping
		requireServiceMissing(t, a, "redis")

		// Ensure checks were removed
		requireCheckMissing(t, a, "service:redis:1")
		requireCheckMissing(t, a, "service:redis:2")
		requireCheckMissingMap(t, a.checkTTLs, "service:redis:1")
		requireCheckMissingMap(t, a.checkTTLs, "service:redis:2")

		// check the mysql service is unnafected
		requireCheckExistsMap(t, a.checkTTLs, "service:mysql:1")
		requireCheckExists(t, a, "service:mysql:1")
		requireCheckExistsMap(t, a.checkTTLs, "service:mysql:2")
		requireCheckExists(t, a, "service:mysql:2")
	}
}

func TestAgent_RemoveServiceRemovesAllChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RemoveServiceRemovesAllChecks(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RemoveServiceRemovesAllChecks(t, "enable_central_service_config = true")
	})
}

func testAgent_RemoveServiceRemovesAllChecks(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		node_name = "node1"
	`+extraHCL)
	defer a.Shutdown()
	svc := &structs.NodeService{ID: "redis", Service: "redis", Port: 8000, EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition()}
	chk1 := &structs.CheckType{CheckID: "chk1", Name: "chk1", TTL: time.Minute}
	chk2 := &structs.CheckType{CheckID: "chk2", Name: "chk2", TTL: 2 * time.Minute}
	hchk1 := &structs.HealthCheck{
		Node:           "node1",
		CheckID:        "chk1",
		Name:           "chk1",
		Status:         "critical",
		ServiceID:      "redis",
		ServiceName:    "redis",
		Type:           "ttl",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	hchk2 := &structs.HealthCheck{Node: "node1",
		CheckID:        "chk2",
		Name:           "chk2",
		Status:         "critical",
		ServiceID:      "redis",
		ServiceName:    "redis",
		Type:           "ttl",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// register service with chk1
	if err := a.addServiceFromSource(svc, []*structs.CheckType{chk1}, false, "", ConfigSourceLocal); err != nil {
		t.Fatal("Failed to register service", err)
	}

	// verify chk1 exists
	requireCheckExists(t, a, "chk1")

	// update the service with chk2
	if err := a.addServiceFromSource(svc, []*structs.CheckType{chk2}, false, "", ConfigSourceLocal); err != nil {
		t.Fatal("Failed to update service", err)
	}

	// check that both checks are there
	require.Equal(t, hchk1, getCheck(a, "chk1"))
	require.Equal(t, hchk2, getCheck(a, "chk2"))

	// Remove service
	if err := a.RemoveService(structs.NewServiceID("redis", nil)); err != nil {
		t.Fatal("Failed to remove service", err)
	}

	// Check that both checks are gone
	requireCheckMissing(t, a, "chk1")
	requireCheckMissing(t, a, "chk2")
}

// TestAgent_IndexChurn is designed to detect a class of issues where
// we would have unnecessary catalog churn from anti-entropy. See issues
// #3259, #3642, #3845, and #3866.
func TestAgent_IndexChurn(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	t.Run("no tags", func(t *testing.T) {
		verifyIndexChurn(t, nil)
	})

	t.Run("with tags", func(t *testing.T) {
		verifyIndexChurn(t, []string{"foo", "bar"})
	})
}

// verifyIndexChurn registers some things and runs anti-entropy a bunch of times
// in a row to make sure there are no index bumps.
func verifyIndexChurn(t *testing.T, tags []string) {
	t.Helper()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	weights := &structs.Weights{
		Passing: 1,
		Warning: 1,
	}
	// Ensure we have a leader before we start adding the services
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
		Tags:    tags,
		Weights: weights,
	}
	if err := a.addServiceFromSource(svc, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	chk := &structs.HealthCheck{
		CheckID:   "redis-check",
		Name:      "Service-level check",
		ServiceID: "redis",
		Status:    api.HealthCritical,
	}
	chkt := &structs.CheckType{
		TTL: time.Hour,
	}
	if err := a.AddCheck(chk, chkt, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	chk = &structs.HealthCheck{
		CheckID: "node-check",
		Name:    "Node-level check",
		Status:  api.HealthCritical,
	}
	chkt = &structs.CheckType{
		TTL: time.Hour,
	}
	if err := a.AddCheck(chk, chkt, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := a.sync.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	args := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "redis",
	}
	var before structs.IndexedCheckServiceNodes

	// This sleep is so that the serfHealth check is added to the agent
	// A value of 375ms is sufficient enough time to ensure the serfHealth
	// check is added to an agent. 500ms so that we don't see flakiness ever.
	time.Sleep(500 * time.Millisecond)

	if err := a.RPC("Health.ServiceNodes", args, &before); err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, name := range before.Nodes[0].Checks {
		a.logger.Debug("Registered node", "node", name.Name)
	}
	if got, want := len(before.Nodes), 1; got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	if got, want := len(before.Nodes[0].Checks), 3; /* incl. serfHealth */ got != want {
		t.Fatalf("got %d want %d", got, want)
	}

	for i := 0; i < 10; i++ {
		a.logger.Info("Sync in progress", "iteration", i+1)
		if err := a.sync.State.SyncFull(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	// If this test fails here this means that the Consul-X-Index
	// has changed for the RPC, which means that idempotent ops
	// are not working as intended.
	var after structs.IndexedCheckServiceNodes
	if err := a.RPC("Health.ServiceNodes", args, &after); err != nil {
		t.Fatalf("err: %v", err)
	}
	require.Equal(t, before, after)
}

func TestAgent_AddCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	sChk := requireCheckExists(t, a, "mem")

	// Ensure our check is in the right state
	if sChk.Status != api.HealthCritical {
		t.Fatalf("check not critical")
	}

	// Ensure a TTL is setup
	requireCheckExistsMap(t, a.checkMonitors, "mem")
}

func TestAgent_AddCheck_StartPassing(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthPassing,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	sChk := requireCheckExists(t, a, "mem")

	// Ensure our check is in the right state
	if sChk.Status != api.HealthPassing {
		t.Fatalf("check not passing")
	}

	// Ensure a TTL is setup
	requireCheckExistsMap(t, a.checkMonitors, "mem")
}

func TestAgent_AddCheck_MinInterval(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   time.Microsecond,
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	requireCheckExists(t, a, "mem")

	// Ensure a TTL is setup
	if mon, ok := a.checkMonitors[structs.NewCheckID("mem", nil)]; !ok {
		t.Fatalf("missing mem monitor")
	} else if mon.Interval != checks.MinInterval {
		t.Fatalf("bad mem monitor interval")
	}
}

func TestAgent_AddCheck_MissingService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "baz",
		Name:      "baz check 1",
		ServiceID: "baz",
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   time.Microsecond,
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err == nil || err.Error() != fmt.Sprintf("ServiceID %q does not exist", structs.ServiceIDString("baz", nil)) {
		t.Fatalf("expected service id error, got: %v", err)
	}
}

func TestAgent_AddCheck_RestoreState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Create some state and persist it
	ttl := &checks.CheckTTL{
		CheckID: structs.NewCheckID("baz", nil),
		TTL:     time.Minute,
	}
	err := a.persistCheckState(ttl, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Build and register the check definition and initial state
	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "baz",
		Name:    "baz check 1",
	}
	chk := &structs.CheckType{
		TTL: time.Minute,
	}
	err = a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check status was restored during registration
	check := requireCheckExists(t, a, "baz")
	if check.Status != api.HealthPassing {
		t.Fatalf("bad: %#v", check)
	}
	if check.Output != "yup" {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_AddCheck_ExecDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}

	// Ensure we don't have a check mapping
	requireCheckMissing(t, a, "mem")

	err = a.AddCheck(health, chk, false, "", ConfigSourceRemote)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}

	// Ensure we don't have a check mapping
	requireCheckMissing(t, a, "mem")
}

func TestAgent_AddCheck_ExecRemoteDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, `
		enable_local_script_checks = true
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceRemote)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent from remote calls") {
		t.Fatalf("err: %v", err)
	}

	// Ensure we don't have a check mapping
	requireCheckMissing(t, a, "mem")
}

func TestAgent_AddCheck_GRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "grpchealth",
		Name:    "grpc health checking protocol",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		GRPC:     "localhost:12345/package.Service",
		Interval: 15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	sChk := requireCheckExists(t, a, "grpchealth")

	// Ensure our check is in the right state
	if sChk.Status != api.HealthCritical {
		t.Fatalf("check not critical")
	}

	// Ensure a check is setup
	requireCheckExistsMap(t, a.checkGRPCs, "grpchealth")
}

func TestAgent_RestoreServiceWithAliasCheck(t *testing.T) {
	// t.Parallel() don't even think about making this parallel

	// This test is very contrived and tests for the absence of race conditions
	// related to the implementation of alias checks. As such it is slow,
	// serial, full of sleeps and retries, and not generally a great test to
	// run all of the time.
	//
	// That said it made it incredibly easy to root out various race conditions
	// quite successfully.
	//
	// The original set of races was between:
	//
	//   - agent startup reloading Services and Checks from disk
	//   - API requests to also re-register those same Services and Checks
	//   - the goroutines for the as-yet-to-be-stopped CheckAlias goroutines

	if os.Getenv("SLOWTEST") != "1" {
		t.Skip("skipping slow test; set SLOWTEST=1 to run")
		return
	}

	// We do this so that the agent logs and the informational messages from
	// the test itself are interwoven properly.
	logf := func(t *testing.T, a *TestAgent, format string, args ...interface{}) {
		a.logger.Info("testharness: " + fmt.Sprintf(format, args...))
	}

	cfg := `
		server = false
		bootstrap = false
	    enable_central_service_config = false
	`
	a := StartTestAgent(t, TestAgent{HCL: cfg})
	defer a.Shutdown()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})
	testHTTPServer := httptest.NewServer(handler)
	t.Cleanup(testHTTPServer.Close)

	registerServicesAndChecks := func(t *testing.T, a *TestAgent) {
		// add one persistent service with a simple check
		require.NoError(t, a.addServiceFromSource(
			&structs.NodeService{
				ID:      "ping",
				Service: "ping",
				Port:    8000,
			},
			[]*structs.CheckType{
				{
					HTTP:     testHTTPServer.URL,
					Method:   "GET",
					Interval: 5 * time.Second,
					Timeout:  1 * time.Second,
				},
			},
			true, "", ConfigSourceLocal,
		))

		// add one persistent sidecar service with an alias check in the manner
		// of how sidecar_service would add it
		require.NoError(t, a.addServiceFromSource(
			&structs.NodeService{
				ID:      "ping-sidecar-proxy",
				Service: "ping-sidecar-proxy",
				Port:    9000,
			},
			[]*structs.CheckType{
				{
					Name:         "Connect Sidecar Aliasing ping",
					AliasService: "ping",
				},
			},
			true, "", ConfigSourceLocal,
		))
	}

	retryUntilCheckState := func(t *testing.T, a *TestAgent, checkID string, expectedStatus string) {
		t.Helper()
		retry.Run(t, func(r *retry.R) {
			chk := requireCheckExists(t, a, types.CheckID(checkID))
			if chk.Status != expectedStatus {
				logf(t, a, "check=%q expected status %q but got %q", checkID, expectedStatus, chk.Status)
				r.Fatalf("check=%q expected status %q but got %q", checkID, expectedStatus, chk.Status)
			}
			logf(t, a, "check %q has reached desired status %q", checkID, expectedStatus)
		})
	}

	registerServicesAndChecks(t, a)

	time.Sleep(1 * time.Second)

	retryUntilCheckState(t, a, "service:ping", api.HealthPassing)
	retryUntilCheckState(t, a, "service:ping-sidecar-proxy", api.HealthPassing)

	logf(t, a, "==== POWERING DOWN ORIGINAL ====")

	require.NoError(t, a.Shutdown())

	time.Sleep(1 * time.Second)

	futureHCL := cfg + `
node_id = "` + string(a.Config.NodeID) + `"
node_name = "` + a.Config.NodeName + `"
	`

	restartOnce := func(idx int, t *testing.T) {
		t.Helper()

		// Reload and retain former NodeID and data directory.
		a2 := StartTestAgent(t, TestAgent{HCL: futureHCL, DataDir: a.DataDir})
		defer a2.Shutdown()
		a = nil

		// reregister during standup; we use an adjustable timing to try and force a race
		sleepDur := time.Duration(idx+1) * 500 * time.Millisecond
		time.Sleep(sleepDur)
		logf(t, a2, "re-registering checks and services after a delay of %v", sleepDur)
		for i := 0; i < 20; i++ { // RACE RACE RACE!
			registerServicesAndChecks(t, a2)
			time.Sleep(50 * time.Millisecond)
		}

		time.Sleep(1 * time.Second)

		retryUntilCheckState(t, a2, "service:ping", api.HealthPassing)

		logf(t, a2, "giving the alias check a chance to notice...")
		time.Sleep(5 * time.Second)

		retryUntilCheckState(t, a2, "service:ping-sidecar-proxy", api.HealthPassing)
	}

	for i := 0; i < 20; i++ {
		name := "restart-" + strconv.Itoa(i)
		ok := t.Run(name, func(t *testing.T) {
			restartOnce(i, t)
		})
		require.True(t, ok, name+" failed")
	}
}

func TestAgent_Alias_AddRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	cid := structs.NewCheckID("aliashealth", nil)

	testutil.RunStep(t, "add check", func(t *testing.T) {
		health := &structs.HealthCheck{
			Node:    "foo",
			CheckID: cid.ID,
			Name:    "Alias health check",
			Status:  api.HealthCritical,
		}
		chk := &structs.CheckType{
			AliasService: "foo",
		}
		err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
		require.NoError(t, err)

		sChk := requireCheckExists(t, a, cid.ID)
		require.Equal(t, api.HealthCritical, sChk.Status)

		chkImpl, ok := a.checkAliases[cid]
		require.True(t, ok, "missing aliashealth check")
		require.Equal(t, "", chkImpl.RPCReq.Token)

		cs := a.State.CheckState(cid)
		require.NotNil(t, cs)
		require.Equal(t, "", cs.Token)
	})

	testutil.RunStep(t, "remove check", func(t *testing.T) {
		require.NoError(t, a.RemoveCheck(cid, false))

		requireCheckMissing(t, a, cid.ID)
		requireCheckMissingMap(t, a.checkAliases, cid.ID)
	})
}

func TestAgent_AddCheck_Alias_setToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "aliashealth",
		Name:    "Alias health check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		AliasService: "foo",
	}
	err := a.AddCheck(health, chk, false, "foo", ConfigSourceLocal)
	require.NoError(t, err)

	cs := a.State.CheckState(structs.NewCheckID("aliashealth", nil))
	require.NotNil(t, cs)
	require.Equal(t, "foo", cs.Token)

	chkImpl, ok := a.checkAliases[structs.NewCheckID("aliashealth", nil)]
	require.True(t, ok, "missing aliashealth check")
	require.Equal(t, "foo", chkImpl.RPCReq.Token)
}

func TestAgent_AddCheck_Alias_userToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, `
acl_token = "hello"
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "aliashealth",
		Name:    "Alias health check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		AliasService: "foo",
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	require.NoError(t, err)

	cs := a.State.CheckState(structs.NewCheckID("aliashealth", nil))
	require.NotNil(t, cs)
	require.Equal(t, "", cs.Token) // State token should still be empty

	chkImpl, ok := a.checkAliases[structs.NewCheckID("aliashealth", nil)]
	require.True(t, ok, "missing aliashealth check")
	require.Equal(t, "hello", chkImpl.RPCReq.Token) // Check should use the token
}

func TestAgent_AddCheck_Alias_userAndSetToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, `
acl_token = "hello"
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "aliashealth",
		Name:    "Alias health check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		AliasService: "foo",
	}
	err := a.AddCheck(health, chk, false, "goodbye", ConfigSourceLocal)
	require.NoError(t, err)

	cs := a.State.CheckState(structs.NewCheckID("aliashealth", nil))
	require.NotNil(t, cs)
	require.Equal(t, "goodbye", cs.Token)

	chkImpl, ok := a.checkAliases[structs.NewCheckID("aliashealth", nil)]
	require.True(t, ok, "missing aliashealth check")
	require.Equal(t, "goodbye", chkImpl.RPCReq.Token)
}

func TestAgent_RemoveCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	// Remove check that doesn't exist
	if err := a.RemoveCheck(structs.NewCheckID("mem", nil), false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove without an ID
	if err := a.RemoveCheck(structs.NewCheckID("", nil), false); err == nil {
		t.Fatalf("should have errored")
	}

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove check
	if err := a.RemoveCheck(structs.NewCheckID("mem", nil), false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	requireCheckMissing(t, a, "mem")

	// Ensure a TTL is setup
	requireCheckMissingMap(t, a.checkMonitors, "mem")
}

func TestAgent_HTTPCheck_TLSSkipVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "GOOD")
	})
	server := httptest.NewTLSServer(handler)
	defer server.Close()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "tls",
		Name:    "tls check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		HTTP:          server.URL,
		Interval:      20 * time.Millisecond,
		TLSSkipVerify: true,
	}

	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		status := getCheck(a, "tls")
		if status.Status != api.HealthPassing {
			r.Fatalf("bad: %v", status.Status)
		}
		if !strings.Contains(status.Output, "GOOD") {
			r.Fatalf("bad: %v", status.Output)
		}
	})

}

func TestAgent_HTTPCheck_EnableAgentTLSForChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	run := func(t *testing.T, ca string) {
		a := StartTestAgent(t, TestAgent{
			UseTLS: true,
			HCL: `
				enable_agent_tls_for_checks = true

				verify_incoming = true
				server_name = "consul.test"
				key_file = "../test/client_certs/server.key"
				cert_file = "../test/client_certs/server.crt"
			` + ca,
		})
		defer a.Shutdown()

		health := &structs.HealthCheck{
			Node:    "foo",
			CheckID: "tls",
			Name:    "tls check",
			Status:  api.HealthCritical,
		}

		addr, err := firstAddr(a.Agent.apiServers, "https")
		require.NoError(t, err)
		url := fmt.Sprintf("https://%s/v1/agent/self", addr.String())
		chk := &structs.CheckType{
			HTTP:     url,
			Interval: 20 * time.Millisecond,
		}

		err = a.AddCheck(health, chk, false, "", ConfigSourceLocal)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		retry.Run(t, func(r *retry.R) {
			status := getCheck(a, "tls")
			if status.Status != api.HealthPassing {
				r.Fatalf("bad: %v", status.Status)
			}
			if !strings.Contains(status.Output, "200 OK") {
				r.Fatalf("bad: %v", status.Output)
			}
		})
	}

	// We need to test both methods of passing the CA info to ensure that
	// we propagate all the fields correctly. All the other fields are
	// covered by the HCL in the test run function.
	tests := []struct {
		desc   string
		config string
	}{
		{"ca_file", `ca_file = "../test/client_certs/rootca.crt"`},
		{"ca_path", `ca_path = "../test/client_certs/path"`},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			run(t, tt.config)
		})
	}
}

func TestAgent_updateTTLCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	checkBufSize := 100
	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		TTL:           15 * time.Second,
		OutputMaxSize: checkBufSize,
	}

	// Add check and update it.
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := a.updateTTLCheck(structs.NewCheckID("mem", nil), api.HealthPassing, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping.
	status := getCheck(a, "mem")
	if status.Status != api.HealthPassing {
		t.Fatalf("bad: %v", status)
	}
	if status.Output != "foo" {
		t.Fatalf("bad: %v", status)
	}

	if err := a.updateTTLCheck(structs.NewCheckID("mem", nil), api.HealthCritical, strings.Repeat("--bad-- ", 5*checkBufSize)); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping.
	status = getCheck(a, "mem")
	if status.Status != api.HealthCritical {
		t.Fatalf("bad: %v", status)
	}
	if len(status.Output) > checkBufSize*2 {
		t.Fatalf("bad: %v", len(status.Output))
	}
}

func TestAgent_PersistService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_PersistService(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_PersistService(t, "enable_central_service_config = true")
	})
}

func testAgent_PersistService(t *testing.T, extraHCL string) {
	t.Helper()

	cfg := `
		server = false
		bootstrap = false
	` + extraHCL
	a := StartTestAgent(t, TestAgent{HCL: cfg})
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	file := filepath.Join(a.Config.DataDir, servicesDir, structs.NewServiceID(svc.ID, nil).StringHashSHA256())

	// Check is not persisted unless requested
	if err := a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should not persist")
	}

	// Persists to file if requested
	if err := a.addServiceFromSource(svc, nil, true, "mytoken", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}
	expected, err := json.Marshal(persistedService{
		Token:   "mytoken",
		Service: svc,
		Source:  "local",
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(expected, content) {
		t.Fatalf("bad: %s", string(content))
	}

	// Updates service definition on disk
	svc.Port = 8001
	if err := a.addServiceFromSource(svc, nil, true, "mytoken", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	expected, err = json.Marshal(persistedService{
		Token:   "mytoken",
		Service: svc,
		Source:  "local",
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	content, err = os.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(expected, content) {
		t.Fatalf("bad: %s", string(content))
	}
	a.Shutdown()

	// Should load it back during later start
	a2 := StartTestAgent(t, TestAgent{HCL: cfg, DataDir: a.DataDir})
	defer a2.Shutdown()

	restored := a2.State.ServiceState(structs.NewServiceID(svc.ID, nil))
	if restored == nil {
		t.Fatalf("service %q missing", svc.ID)
	}
	if got, want := restored.Token, "mytoken"; got != want {
		t.Fatalf("got token %q want %q", got, want)
	}
	if got, want := restored.Service.Port, 8001; got != want {
		t.Fatalf("got port %d want %d", got, want)
	}
}

func TestAgent_persistedService_compat(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_persistedService_compat(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_persistedService_compat(t, "enable_central_service_config = true")
	})
}

func testAgent_persistedService_compat(t *testing.T, extraHCL string) {
	t.Helper()

	// Tests backwards compatibility of persisted services from pre-0.5.1
	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:              "redis",
		Service:         "redis",
		Tags:            []string{"foo"},
		Port:            8000,
		TaggedAddresses: map[string]structs.ServiceAddress{},
		Weights:         &structs.Weights{Passing: 1, Warning: 1},
		EnterpriseMeta:  *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// Encode the NodeService directly. This is what previous versions
	// would serialize to the file (without the wrapper)
	encoded, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Write the content to the file
	file := filepath.Join(a.Config.DataDir, servicesDir, structs.NewServiceID(svc.ID, nil).StringHashSHA256())
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := os.WriteFile(file, encoded, 0600); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Load the services
	if err := a.loadServices(a.Config, nil); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the service was restored
	result := requireServiceExists(t, a, "redis")
	require.Equal(t, svc, result)
}

func TestAgent_persistedService_compat_hash(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_persistedService_compat_hash(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_persistedService_compat_hash(t, "enable_central_service_config = true")
	})

}

func testAgent_persistedService_compat_hash(t *testing.T, extraHCL string) {
	t.Helper()

	// Tests backwards compatibility of persisted services from pre-0.5.1
	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:              "redis",
		Service:         "redis",
		Tags:            []string{"foo"},
		Port:            8000,
		TaggedAddresses: map[string]structs.ServiceAddress{},
		Weights:         &structs.Weights{Passing: 1, Warning: 1},
		EnterpriseMeta:  *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// Encode the NodeService directly. This is what previous versions
	// would serialize to the file (without the wrapper)
	encoded, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Write the content to the file using the old md5 based path
	file := filepath.Join(a.Config.DataDir, servicesDir, stringHashMD5(svc.ID))
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := os.WriteFile(file, encoded, 0600); err != nil {
		t.Fatalf("err: %s", err)
	}

	wrapped := persistedServiceConfig{
		ServiceID:      "redis",
		Defaults:       &structs.ServiceConfigResponse{},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	encodedConfig, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	configFile := filepath.Join(a.Config.DataDir, serviceConfigDir, stringHashMD5(svc.ID))
	if err := os.MkdirAll(filepath.Dir(configFile), 0700); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := os.WriteFile(configFile, encodedConfig, 0600); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Load the services
	if err := a.loadServices(a.Config, nil); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the service was restored
	result := requireServiceExists(t, a, "redis")
	require.Equal(t, svc, result)
}

// Exists for backwards compatibility testing
func stringHashMD5(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func TestAgent_PurgeService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_PurgeService(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_PurgeService(t, "enable_central_service_config = true")
	})
}

func testAgent_PurgeService(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	file := filepath.Join(a.Config.DataDir, servicesDir, structs.NewServiceID(svc.ID, nil).StringHashSHA256())
	if err := a.addServiceFromSource(svc, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Exists
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Not removed
	if err := a.removeService(structs.NewServiceID(svc.ID, nil), false); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Re-add the service
	if err := a.addServiceFromSource(svc, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Removed
	if err := a.removeService(structs.NewServiceID(svc.ID, nil), true); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("bad: %#v", err)
	}
}

func TestAgent_PurgeServiceOnDuplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_PurgeServiceOnDuplicate(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_PurgeServiceOnDuplicate(t, "enable_central_service_config = true")
	})
}

func testAgent_PurgeServiceOnDuplicate(t *testing.T, extraHCL string) {
	t.Helper()

	cfg := `
		server = false
		bootstrap = false
	` + extraHCL
	a := StartTestAgent(t, TestAgent{HCL: cfg})
	defer a.Shutdown()

	svc1 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// First persist the service
	require.NoError(t, a.addServiceFromSource(svc1, nil, true, "", ConfigSourceLocal))
	a.Shutdown()

	// Try bringing the agent back up with the service already
	// existing in the config
	a2 := StartTestAgent(t, TestAgent{Name: "Agent2", HCL: cfg + `
		service = {
			id = "redis"
			name = "redis"
			tags = ["bar"]
			port = 9000
		}
	`, DataDir: a.DataDir})
	defer a2.Shutdown()

	sid := svc1.CompoundServiceID()
	file := filepath.Join(a.Config.DataDir, servicesDir, sid.StringHashSHA256())
	_, err := os.Stat(file)
	require.Error(t, err, "should have removed persisted service")
	result := requireServiceExists(t, a, "redis")
	require.NotEqual(t, []string{"bar"}, result.Tags)
	require.NotEqual(t, 9000, result.Port)
}

func TestAgent_PersistCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	cfg := `
		server = false
		bootstrap = false
		enable_script_checks = true
	`
	a := StartTestAgent(t, TestAgent{HCL: cfg})
	defer a.Shutdown()

	check := &structs.HealthCheck{
		Node:    a.config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  api.HealthPassing,
	}
	chkType := &structs.CheckType{
		ScriptArgs: []string{"/bin/true"},
		Interval:   10 * time.Second,
	}

	cid := check.CompoundCheckID()
	file := filepath.Join(a.Config.DataDir, checksDir, cid.StringHashSHA256())

	// Not persisted if not requested
	require.NoError(t, a.AddCheck(check, chkType, false, "", ConfigSourceLocal))
	_, err := os.Stat(file)
	require.Error(t, err, "should not persist")

	// Should persist if requested
	require.NoError(t, a.AddCheck(check, chkType, true, "mytoken", ConfigSourceLocal))
	_, err = os.Stat(file)
	require.NoError(t, err)

	expected, err := json.Marshal(persistedCheck{
		Check:   check,
		ChkType: chkType,
		Token:   "mytoken",
		Source:  "local",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(file)
	require.NoError(t, err)

	require.Equal(t, expected, content)

	// Updates the check definition on disk
	check.Name = "mem1"
	require.NoError(t, a.AddCheck(check, chkType, true, "mytoken", ConfigSourceLocal))
	expected, err = json.Marshal(persistedCheck{
		Check:   check,
		ChkType: chkType,
		Token:   "mytoken",
		Source:  "local",
	})
	require.NoError(t, err)
	content, err = os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, expected, content)
	a.Shutdown()

	// Should load it back during later start
	a2 := StartTestAgent(t, TestAgent{Name: "Agent2", HCL: cfg, DataDir: a.DataDir})
	defer a2.Shutdown()

	result := requireCheckExists(t, a2, check.CheckID)
	require.Equal(t, api.HealthCritical, result.Status)
	require.Equal(t, "mem1", result.Name)

	// Should have restored the monitor
	requireCheckExistsMap(t, a2.checkMonitors, check.CheckID)
	chkState := a2.State.CheckState(structs.NewCheckID(check.CheckID, nil))
	require.NotNil(t, chkState)
	require.Equal(t, "mytoken", chkState.Token)
}

func TestAgent_PurgeCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	check := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  api.HealthPassing,
	}

	file := filepath.Join(a.Config.DataDir, checksDir, checkIDHash(check.CheckID))
	if err := a.AddCheck(check, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Not removed
	if err := a.RemoveCheck(structs.NewCheckID(check.CheckID, nil), false); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Removed
	if err := a.RemoveCheck(structs.NewCheckID(check.CheckID, nil), true); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("bad: %#v", err)
	}
}

func TestAgent_PurgeCheckOnDuplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	nodeID := NodeID()
	a := StartTestAgent(t, TestAgent{
		HCL: `
	    node_id = "` + nodeID + `"
	    node_name = "Node ` + nodeID + `"
		server = false
		bootstrap = false
		enable_script_checks = true
	`})
	defer a.Shutdown()

	check1 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "mem",
		Name:           "memory check",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// First persist the check
	if err := a.AddCheck(check1, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	a.Shutdown()

	// Start again with the check registered in config
	a2 := StartTestAgent(t, TestAgent{
		Name:    "Agent2",
		DataDir: a.DataDir,
		HCL: `
	    node_id = "` + nodeID + `"
	    node_name = "Node ` + nodeID + `"
		server = false
		bootstrap = false
		enable_script_checks = true
		check = {
			id = "mem"
			name = "memory check"
			notes = "my cool notes"
			args = ["/bin/check-redis.py"]
			interval = "30s"
			timeout = "5s"
		}
	`})
	defer a2.Shutdown()

	cid := check1.CompoundCheckID()
	file := filepath.Join(a.DataDir, checksDir, cid.StringHashSHA256())
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should have removed persisted check")
	}
	result := requireCheckExists(t, a2, "mem")
	expected := &structs.HealthCheck{
		Node:           a2.Config.NodeName,
		CheckID:        "mem",
		Name:           "memory check",
		Status:         api.HealthCritical,
		Notes:          "my cool notes",
		Interval:       "30s",
		Timeout:        "5s",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	require.Equal(t, expected, result)
}

func TestAgent_DeregisterPersistedSidecarAfterRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	nodeID := NodeID()
	a := StartTestAgent(t, TestAgent{
		HCL: `
	    node_id = "` + nodeID + `"
	    node_name = "Node ` + nodeID + `"
		server = false
		bootstrap = false
		enable_central_service_config = false
	`})
	defer a.Shutdown()

	srv := &structs.NodeService{
		ID:      "svc",
		Service: "svc",
		Weights: &structs.Weights{
			Passing: 2,
			Warning: 1,
		},
		Tags:           []string{"tag2"},
		Port:           8200,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),

		Connect: structs.ServiceConnect{
			SidecarService: &structs.ServiceDefinition{},
		},
	}

	connectSrv, _, _, err := sidecarServiceFromNodeService(srv, "")
	require.NoError(t, err)

	// First persist the check
	err = a.addServiceFromSource(srv, nil, true, "", ConfigSourceLocal)
	require.NoError(t, err)
	err = a.addServiceFromSource(connectSrv, nil, true, "", ConfigSourceLocal)
	require.NoError(t, err)

	// check both services were registered
	require.NotNil(t, a.State.Service(srv.CompoundServiceID()))
	require.NotNil(t, a.State.Service(connectSrv.CompoundServiceID()))

	a.Shutdown()

	// Start again with the check registered in config
	a2 := StartTestAgent(t, TestAgent{
		Name:    "Agent2",
		DataDir: a.DataDir,
		HCL: `
	    node_id = "` + nodeID + `"
	    node_name = "Node ` + nodeID + `"
		server = false
		bootstrap = false
		enable_central_service_config = false
	`})
	defer a2.Shutdown()

	// check both services were restored
	require.NotNil(t, a2.State.Service(srv.CompoundServiceID()))
	require.NotNil(t, a2.State.Service(connectSrv.CompoundServiceID()))

	err = a2.RemoveService(srv.CompoundServiceID())
	require.NoError(t, err)

	// check both services were deregistered
	require.Nil(t, a2.State.Service(srv.CompoundServiceID()))
	require.Nil(t, a2.State.Service(connectSrv.CompoundServiceID()))
}

func TestAgent_loadChecks_token(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		check = {
			id = "rabbitmq"
			name = "rabbitmq"
			token = "abc123"
			ttl = "10s"
		}
	`)
	defer a.Shutdown()

	requireCheckExists(t, a, "rabbitmq")
	require.Equal(t, "abc123", a.State.CheckToken(structs.NewCheckID("rabbitmq", nil)))
}

func TestAgent_unloadChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      api.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	if err := a.AddCheck(check1, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}

	requireCheckExists(t, a, check1.CheckID)

	// Unload all of the checks
	if err := a.unloadChecks(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure it was unloaded
	requireCheckMissing(t, a, check1.CheckID)
}

func TestAgent_loadServices_token(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_token(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_token(t, "enable_central_service_config = true")
	})
}

func testAgent_loadServices_token(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
		}
	`+extraHCL)
	defer a.Shutdown()

	requireServiceExists(t, a, "rabbitmq")
	if token := a.State.ServiceToken(structs.NewServiceID("rabbitmq", nil)); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_loadServices_sidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_sidecar(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_sidecar(t, "enable_central_service_config = true")
	})
}

func testAgent_loadServices_sidecar(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
			connect = {
				sidecar_service {}
			}
		}
	`+extraHCL)
	defer a.Shutdown()

	svc := requireServiceExists(t, a, "rabbitmq")
	if token := a.State.ServiceToken(structs.NewServiceID("rabbitmq", nil)); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
	sidecarSvc := requireServiceExists(t, a, "rabbitmq-sidecar-proxy")
	if token := a.State.ServiceToken(structs.NewServiceID("rabbitmq-sidecar-proxy", nil)); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}

	// Verify default checks have been added
	wantChecks := sidecarDefaultChecks(sidecarSvc.ID, sidecarSvc.Address, sidecarSvc.Proxy.LocalServiceAddress, sidecarSvc.Port)
	gotChecks := a.State.ChecksForService(sidecarSvc.CompoundServiceID(), true)
	gotChkNames := make(map[string]types.CheckID)
	for _, check := range gotChecks {
		requireCheckExists(t, a, check.CheckID)
		gotChkNames[check.Name] = check.CheckID
	}
	for _, check := range wantChecks {
		chkName := check.Name
		require.NotNil(t, gotChkNames[chkName])
	}

	// Sanity check rabbitmq service should NOT have sidecar info in state since
	// it's done it's job and should be a registration syntax sugar only.
	assert.Nil(t, svc.Connect.SidecarService)
}

func TestAgent_loadServices_sidecarSeparateToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_sidecarSeparateToken(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_sidecarSeparateToken(t, "enable_central_service_config = true")
	})
}

func testAgent_loadServices_sidecarSeparateToken(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
			connect = {
				sidecar_service {
					token = "789xyz"
				}
			}
		}
	`+extraHCL)
	defer a.Shutdown()

	requireServiceExists(t, a, "rabbitmq")
	if token := a.State.ServiceToken(structs.NewServiceID("rabbitmq", nil)); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
	requireServiceExists(t, a, "rabbitmq-sidecar-proxy")
	if token := a.State.ServiceToken(structs.NewServiceID("rabbitmq-sidecar-proxy", nil)); token != "789xyz" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_loadServices_sidecarInheritMeta(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_sidecarInheritMeta(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_sidecarInheritMeta(t, "enable_central_service_config = true")
	})
}

func testAgent_loadServices_sidecarInheritMeta(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			tags = ["a", "b"],
			meta = {
				environment = "prod"
			}
			connect = {
				sidecar_service {

				}
			}
		}
	`+extraHCL)
	defer a.Shutdown()

	svc := requireServiceExists(t, a, "rabbitmq")
	require.Len(t, svc.Tags, 2)
	require.Len(t, svc.Meta, 1)

	sidecar := requireServiceExists(t, a, "rabbitmq-sidecar-proxy")
	require.ElementsMatch(t, svc.Tags, sidecar.Tags)
	require.Len(t, sidecar.Meta, 1)
	meta, ok := sidecar.Meta["environment"]
	require.True(t, ok, "missing sidecar service meta")
	require.Equal(t, "prod", meta)
}

func TestAgent_loadServices_sidecarOverrideMeta(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_sidecarOverrideMeta(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_loadServices_sidecarOverrideMeta(t, "enable_central_service_config = true")
	})
}

func testAgent_loadServices_sidecarOverrideMeta(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			tags = ["a", "b"],
			meta = {
				environment = "prod"
			}
			connect = {
				sidecar_service {
					tags = ["foo"],
					meta = {
						environment = "qa"
					}
				}
			}
		}
	`+extraHCL)
	defer a.Shutdown()

	svc := requireServiceExists(t, a, "rabbitmq")
	require.Len(t, svc.Tags, 2)
	require.Len(t, svc.Meta, 1)

	sidecar := requireServiceExists(t, a, "rabbitmq-sidecar-proxy")
	require.Len(t, sidecar.Tags, 1)
	require.Equal(t, "foo", sidecar.Tags[0])
	require.Len(t, sidecar.Meta, 1)
	meta, ok := sidecar.Meta["environment"]
	require.True(t, ok, "missing sidecar service meta")
	require.Equal(t, "qa", meta)
}

func TestAgent_unloadServices(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_unloadServices(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_unloadServices(t, "enable_central_service_config = true")
	})
}

func testAgent_unloadServices(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Register the service
	if err := a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	requireServiceExists(t, a, svc.ID)

	// Unload all services
	if err := a.unloadServices(); err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(a.State.Services(structs.WildcardEnterpriseMetaInDefaultPartition())) != 0 {
		t.Fatalf("should have unloaded services")
	}
}

func TestAgent_Service_MaintenanceMode(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Register the service
	if err := a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	sid := structs.NewServiceID("redis", nil)
	// Enter maintenance mode for the service
	if err := a.EnableServiceMaintenance(sid, "broken", "mytoken"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the critical health check was added
	checkID := serviceMaintCheckID(sid)
	check := a.State.Check(checkID)
	if check == nil {
		t.Fatalf("should have registered critical maintenance check")
	}

	// Check that the token was used to register the check
	if token := a.State.CheckToken(checkID); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got: '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}

	// Leave maintenance mode
	if err := a.DisableServiceMaintenance(sid); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was deregistered

	if found := a.State.Check(checkID); found != nil {
		t.Fatalf("should have deregistered maintenance check")
	}

	// Enter service maintenance mode without providing a reason
	if err := a.EnableServiceMaintenance(sid, "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was registered with the default notes
	check = a.State.Check(checkID)
	if check == nil {
		t.Fatalf("should have registered critical check")
	}
	if check.Notes != defaultServiceMaintReason {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_Service_Reap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// t.Parallel() // timing test. no parallel
	a := StartTestAgent(t, TestAgent{Overrides: `
		check_reap_interval = "50ms"
		check_deregister_interval_min = "0s"
	`})
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	chkTypes := []*structs.CheckType{
		{
			Status:                         api.HealthPassing,
			TTL:                            25 * time.Millisecond,
			DeregisterCriticalServiceAfter: 200 * time.Millisecond,
		},
	}

	// Register the service.
	if err := a.addServiceFromSource(svc, chkTypes, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's there and there's no critical check yet.
	requireServiceExists(t, a, "redis")
	require.Len(t, a.State.CriticalCheckStates(structs.WildcardEnterpriseMetaInDefaultPartition()), 0, "should not have critical checks")

	// Wait for the check TTL to fail but before the check is reaped.
	time.Sleep(100 * time.Millisecond)
	requireServiceExists(t, a, "redis")
	require.Len(t, a.State.CriticalCheckStates(nil), 1, "should have 1 critical check")

	// Pass the TTL.
	if err := a.updateTTLCheck(structs.NewCheckID("service:redis", nil), api.HealthPassing, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
	requireServiceExists(t, a, "redis")
	require.Len(t, a.State.CriticalCheckStates(structs.WildcardEnterpriseMetaInDefaultPartition()), 0, "should not have critical checks")

	// Wait for the check TTL to fail again.
	time.Sleep(100 * time.Millisecond)
	requireServiceExists(t, a, "redis")
	require.Len(t, a.State.CriticalCheckStates(structs.WildcardEnterpriseMetaInDefaultPartition()), 1, "should have 1 critical check")

	// Wait for the reap.
	time.Sleep(400 * time.Millisecond)
	requireServiceMissing(t, a, "redis")
	require.Len(t, a.State.CriticalCheckStates(structs.WildcardEnterpriseMetaInDefaultPartition()), 0, "should not have critical checks")
}

func TestAgent_Service_NoReap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// t.Parallel() // timing test. no parallel
	a := StartTestAgent(t, TestAgent{Overrides: `
		check_reap_interval = "50ms"
		check_deregister_interval_min = "0s"
	`})
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	chkTypes := []*structs.CheckType{
		{
			Status: api.HealthPassing,
			TTL:    25 * time.Millisecond,
		},
	}

	// Register the service.
	if err := a.addServiceFromSource(svc, chkTypes, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's there and there's no critical check yet.
	requireServiceExists(t, a, "redis")
	require.Len(t, a.State.CriticalCheckStates(structs.WildcardEnterpriseMetaInDefaultPartition()), 0)

	// Wait for the check TTL to fail.
	time.Sleep(200 * time.Millisecond)
	requireServiceExists(t, a, "redis")
	require.Len(t, a.State.CriticalCheckStates(structs.WildcardEnterpriseMetaInDefaultPartition()), 1)

	// Wait a while and make sure it doesn't reap.
	time.Sleep(200 * time.Millisecond)
	requireServiceExists(t, a, "redis")
	require.Len(t, a.State.CriticalCheckStates(structs.WildcardEnterpriseMetaInDefaultPartition()), 1)
}

func TestAgent_AddService_restoresSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_AddService_restoresSnapshot(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_AddService_restoresSnapshot(t, "enable_central_service_config = true")
	})
}

func testAgent_AddService_restoresSnapshot(t *testing.T, extraHCL string) {
	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	require.NoError(t, a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal))

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      api.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	require.NoError(t, a.AddCheck(check1, nil, false, "", ConfigSourceLocal))

	// Re-registering the service preserves the state of the check
	chkTypes := []*structs.CheckType{{TTL: 30 * time.Second}}
	require.NoError(t, a.addServiceFromSource(svc, chkTypes, false, "", ConfigSourceLocal))
	check := requireCheckExists(t, a, "service:redis")
	require.Equal(t, api.HealthPassing, check.Status)
}

func TestAgent_AddCheck_restoresSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      api.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	if err := a.AddCheck(check1, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Re-registering the check preserves its state
	check1.Status = ""
	if err := a.AddCheck(check1, &structs.CheckType{TTL: 30 * time.Second}, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}
	check := requireCheckExists(t, a, "service:redis")
	if check.Status != api.HealthPassing {
		t.Fatalf("bad: %s", check.Status)
	}
}

func TestAgent_NodeMaintenanceMode(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Enter maintenance mode for the node
	a.EnableNodeMaintenance("broken", "mytoken")

	// Make sure the critical health check was added
	check := requireCheckExists(t, a, structs.NodeMaint)

	// Check that the token was used to register the check
	if token := a.State.CheckToken(structs.NodeMaintCheckID); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got: '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}

	// Leave maintenance mode
	a.DisableNodeMaintenance()

	// Ensure the check was deregistered
	requireCheckMissing(t, a, structs.NodeMaint)

	// Enter maintenance mode without passing a reason
	a.EnableNodeMaintenance("", "")

	// Make sure the check was registered with the default note
	check = requireCheckExists(t, a, structs.NodeMaint)
	if check.Notes != defaultNodeMaintReason {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_checkStateSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      api.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	if err := a.AddCheck(check1, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Snapshot the state
	snap := a.snapshotCheckState()

	// Unload all of the checks
	if err := a.unloadChecks(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Reload the checks and restore the snapshot.
	if err := a.loadChecks(a.Config, snap); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Search for the check
	out := requireCheckExists(t, a, check1.CheckID)

	// Make sure state was restored
	if out.Status != api.HealthPassing {
		t.Fatalf("should have restored check state")
	}
}

func TestAgent_checkStateSnapshot_backcompat(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      api.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	if err := a.AddCheck(check1, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Snapshot the state
	snap := a.snapshotCheckState()

	// Unload all of the checks
	if err := a.unloadChecks(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Mutate the path to look like the old md5 checksum
	dir := filepath.Join(a.config.DataDir, checksDir)
	new_path := filepath.Join(dir, check1.CompoundCheckID().StringHashSHA256())
	old_path := filepath.Join(dir, check1.CompoundCheckID().StringHashMD5())
	if err := os.Rename(new_path, old_path); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Reload the checks and restore the snapshot.
	if err := a.loadChecks(a.Config, snap); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Search for the check
	out := requireCheckExists(t, a, check1.CheckID)

	// Make sure state was restored
	if out.Status != api.HealthPassing {
		t.Fatalf("should have restored check state")
	}
}

func TestAgent_loadChecks_checkFails(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Persist a health check with an invalid service ID
	check := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "service:redis",
		Name:      "redischeck",
		Status:    api.HealthPassing,
		ServiceID: "nope",
	}
	if err := a.persistCheck(check, nil, ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check to make sure the check was persisted
	checkHash := checkIDHash(check.CheckID)
	checkPath := filepath.Join(a.Config.DataDir, checksDir, checkHash)
	if _, err := os.Stat(checkPath); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Try loading the checks from the persisted files
	if err := a.loadChecks(a.Config, nil); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the erroneous check was purged
	if _, err := os.Stat(checkPath); err == nil {
		t.Fatalf("should have purged check")
	}
}

func TestAgent_persistCheckState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	cid := structs.NewCheckID("check1", nil)
	// Create the TTL check to persist
	check := &checks.CheckTTL{
		CheckID: cid,
		TTL:     10 * time.Minute,
	}

	// Persist some check state for the check
	err := a.persistCheckState(check, api.HealthCritical, "nope")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the persisted file exists and has the content
	file := filepath.Join(a.Config.DataDir, checkStateDir, cid.StringHashSHA256())
	buf, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Decode the state
	var p persistedCheckState
	if err := json.Unmarshal(buf, &p); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the fields
	if p.CheckID != cid.ID {
		t.Fatalf("bad: %#v", p)
	}
	if p.Output != "nope" {
		t.Fatalf("bad: %#v", p)
	}
	if p.Status != api.HealthCritical {
		t.Fatalf("bad: %#v", p)
	}

	// Check the expiration time was set
	if p.Expires < time.Now().Unix() {
		t.Fatalf("bad: %#v", p)
	}
}

func TestAgent_loadCheckState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Create a check whose state will expire immediately
	check := &checks.CheckTTL{
		CheckID: structs.NewCheckID("check1", nil),
		TTL:     0,
	}

	// Persist the check state
	err := a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Try to load the state
	health := &structs.HealthCheck{
		CheckID: "check1",
		Status:  api.HealthCritical,
	}
	if err := a.loadCheckState(health); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Should not have restored the status due to expiration
	if health.Status != api.HealthCritical {
		t.Fatalf("bad: %#v", health)
	}
	if health.Output != "" {
		t.Fatalf("bad: %#v", health)
	}

	// Should have purged the state
	file := filepath.Join(a.Config.DataDir, checksDir, structs.NewCheckID("check1", nil).StringHashSHA256())
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("should have purged state")
	}

	// Set a TTL which will not expire before we check it
	check.TTL = time.Minute
	err = a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Try to load
	if err := a.loadCheckState(health); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Should have restored
	if health.Status != api.HealthPassing {
		t.Fatalf("bad: %#v", health)
	}
	if health.Output != "yup" {
		t.Fatalf("bad: %#v", health)
	}
}

func TestAgent_purgeCheckState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	cid := structs.NewCheckID("check1", nil)
	// No error if the state does not exist
	if err := a.purgeCheckState(cid); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Persist some state to the data dir
	check := &checks.CheckTTL{
		CheckID: cid,
		TTL:     time.Minute,
	}
	err := a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Purge the check state
	if err := a.purgeCheckState(cid); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Removed the file
	file := filepath.Join(a.Config.DataDir, checkStateDir, cid.StringHashSHA256())
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("should have removed file")
	}
}

func TestAgent_GetCoordinate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, ``)
	defer a.Shutdown()

	coords, err := a.GetLANCoordinate()
	require.NoError(t, err)
	expected := lib.CoordinateSet{
		"": &coordinate.Coordinate{
			Error:  1.5,
			Height: 1e-05,
			Vec:    []float64{0, 0, 0, 0, 0, 0, 0, 0},
		},
	}
	require.Equal(t, expected, coords)
}

func TestAgent_reloadWatches(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Normal watch with http addr set, should succeed
	newConf := *a.config
	newConf.Watches = []map[string]interface{}{
		{
			"type": "key",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Should fail to reload with connect watches
	newConf.Watches = []map[string]interface{}{
		{
			"type": "connect_roots",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err == nil || !strings.Contains(err.Error(), "not allowed in agent config") {
		t.Fatalf("bad: %s", err)
	}

	// Should still succeed with only HTTPS addresses
	newConf.HTTPSAddrs = newConf.HTTPAddrs
	newConf.HTTPAddrs = make([]net.Addr, 0)
	newConf.Watches = []map[string]interface{}{
		{
			"type": "key",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Should fail to reload with no http or https addrs
	newConf.HTTPSAddrs = make([]net.Addr, 0)
	newConf.Watches = []map[string]interface{}{
		{
			"type": "key",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err == nil || !strings.Contains(err.Error(), "watch plans require an HTTP or HTTPS endpoint") {
		t.Fatalf("bad: %s", err)
	}
}

func TestAgent_reloadWatchesHTTPS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := TestAgent{UseTLS: true}
	if err := a.Start(t); err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown()

	// Normal watch with http addr set, should succeed
	newConf := *a.config
	newConf.Watches = []map[string]interface{}{
		{
			"type": "key",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err != nil {
		t.Fatalf("bad: %s", err)
	}
}

func TestAgent_SecurityChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	hcl := `
		enable_script_checks = true
	`
	a := &TestAgent{Name: t.Name(), HCL: hcl}
	defer a.Shutdown()

	data := make([]byte, 0, 8192)
	buf := &syncBuffer{b: bytes.NewBuffer(data)}
	a.LogOutput = buf
	assert.NoError(t, a.Start(t))
	assert.Contains(t, buf.String(), "using enable-script-checks without ACLs and without allow_write_http_from is DANGEROUS")
}

type syncBuffer struct {
	lock sync.RWMutex
	b    *bytes.Buffer
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.b.Write(data)
}

func (b *syncBuffer) String() string {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.b.String()
}

func TestAgent_ReloadConfigOutgoingRPCConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	hcl := `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = false
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	tlsConf := a.tlsConfigurator.OutgoingRPCConfig()

	require.True(t, tlsConf.InsecureSkipVerify)
	expectedCaPoolByFile := getExpectedCaPoolByFile(t)
	assertDeepEqual(t, expectedCaPoolByFile, tlsConf.RootCAs, cmpCertPool)
	assertDeepEqual(t, expectedCaPoolByFile, tlsConf.ClientCAs, cmpCertPool)

	hcl = `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_path = "../test/ca_path"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = true
	`
	c := TestConfig(testutil.Logger(t), config.FileSource{Name: t.Name(), Format: "hcl", Data: hcl})
	require.NoError(t, a.reloadConfigInternal(c))
	tlsConf = a.tlsConfigurator.OutgoingRPCConfig()

	require.False(t, tlsConf.InsecureSkipVerify)
	expectedCaPoolByDir := getExpectedCaPoolByDir(t)
	assertDeepEqual(t, expectedCaPoolByDir, tlsConf.RootCAs, cmpCertPool)
	assertDeepEqual(t, expectedCaPoolByDir, tlsConf.ClientCAs, cmpCertPool)
}

func TestAgent_ReloadConfigAndKeepChecksStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_ReloadConfigAndKeepChecksStatus(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_ReloadConfigAndKeepChecksStatus(t, "enable_central_service_config = true")
	})
}

func testAgent_ReloadConfigAndKeepChecksStatus(t *testing.T, extraHCL string) {
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	hcl := `data_dir = "` + dataDir + `"
		enable_local_script_checks=true
		services=[{
		  name="webserver1",
		  check{id="check1", ttl="30s"}
		}] ` + extraHCL
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()

	require.NoError(t, a.updateTTLCheck(structs.NewCheckID("check1", nil), api.HealthPassing, "testing agent reload"))

	// Make sure check is passing before we reload.
	gotChecks := a.State.Checks(nil)
	require.Equal(t, 1, len(gotChecks), "Should have a check registered, but had %#v", gotChecks)
	for id, check := range gotChecks {
		require.Equal(t, "passing", check.Status, "check %q is wrong", id)
	}

	c := TestConfig(testutil.Logger(t), config.FileSource{Name: t.Name(), Format: "hcl", Data: hcl})
	require.NoError(t, a.reloadConfigInternal(c))

	// After reload, should be passing directly (no critical state)
	for id, check := range a.State.Checks(nil) {
		require.Equal(t, "passing", check.Status, "check %q is wrong", id)
	}
}

func TestAgent_ReloadConfigIncomingRPCConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	hcl := `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = false
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	tlsConf := a.tlsConfigurator.IncomingRPCConfig()
	require.NotNil(t, tlsConf.GetConfigForClient)
	tlsConf, err := tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.True(t, tlsConf.InsecureSkipVerify)
	expectedCaPoolByFile := getExpectedCaPoolByFile(t)
	assertDeepEqual(t, expectedCaPoolByFile, tlsConf.RootCAs, cmpCertPool)
	assertDeepEqual(t, expectedCaPoolByFile, tlsConf.ClientCAs, cmpCertPool)

	hcl = `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_path = "../test/ca_path"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = true
	`
	c := TestConfig(testutil.Logger(t), config.FileSource{Name: t.Name(), Format: "hcl", Data: hcl})
	require.NoError(t, a.reloadConfigInternal(c))
	tlsConf, err = tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.False(t, tlsConf.InsecureSkipVerify)
	expectedCaPoolByDir := getExpectedCaPoolByDir(t)
	assertDeepEqual(t, expectedCaPoolByDir, tlsConf.RootCAs, cmpCertPool)
	assertDeepEqual(t, expectedCaPoolByDir, tlsConf.ClientCAs, cmpCertPool)
}

func TestAgent_ReloadConfigTLSConfigFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	hcl := `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = false
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	tlsConf := a.tlsConfigurator.IncomingRPCConfig()

	hcl = `
		data_dir = "` + dataDir + `"
		verify_incoming = true
	`
	c := TestConfig(testutil.Logger(t), config.FileSource{Name: t.Name(), Format: "hcl", Data: hcl})
	require.Error(t, a.reloadConfigInternal(c))
	tlsConf, err := tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.Equal(t, tls.NoClientCert, tlsConf.ClientAuth)

	expectedCaPoolByFile := getExpectedCaPoolByFile(t)
	assertDeepEqual(t, expectedCaPoolByFile, tlsConf.RootCAs, cmpCertPool)
	assertDeepEqual(t, expectedCaPoolByFile, tlsConf.ClientCAs, cmpCertPool)
}

func TestAgent_consulConfig_AutoEncryptAllowTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	hcl := `
		data_dir = "` + dataDir + `"
		verify_incoming = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		auto_encrypt { allow_tls = true }
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	require.True(t, a.consulConfig().AutoEncryptAllowTLS)
}

func TestAgent_ReloadConfigRPCClientConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	hcl := `
		data_dir = "` + dataDir + `"
		server = false
		bootstrap = false
	`
	a := NewTestAgent(t, hcl)

	defaultRPCTimeout := 60 * time.Second
	require.Equal(t, defaultRPCTimeout, a.baseDeps.ConnPool.RPCClientTimeout())

	hcl = `
		data_dir = "` + dataDir + `"
		server = false
		bootstrap = false
		limits {
			rpc_client_timeout = "2m"
		}
	`
	c := TestConfig(testutil.Logger(t), config.FileSource{Name: t.Name(), Format: "hcl", Data: hcl})
	require.NoError(t, a.reloadConfigInternal(c))

	require.Equal(t, 2*time.Minute, a.baseDeps.ConnPool.RPCClientTimeout())
}

func TestAgent_consulConfig_RaftTrailingLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	hcl := `
		raft_trailing_logs = 812345
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	require.Equal(t, uint64(812345), a.consulConfig().RaftConfig.TrailingLogs)
}

func TestAgent_grpcInjectAddr(t *testing.T) {
	tt := []struct {
		name string
		grpc string
		ip   string
		port int
		want string
	}{
		{
			name: "localhost web svc",
			grpc: "localhost:8080/web",
			ip:   "192.168.0.0",
			port: 9090,
			want: "192.168.0.0:9090/web",
		},
		{
			name: "localhost no svc",
			grpc: "localhost:8080",
			ip:   "192.168.0.0",
			port: 9090,
			want: "192.168.0.0:9090",
		},
		{
			name: "ipv4 web svc",
			grpc: "127.0.0.1:8080/web",
			ip:   "192.168.0.0",
			port: 9090,
			want: "192.168.0.0:9090/web",
		},
		{
			name: "ipv4 no svc",
			grpc: "127.0.0.1:8080",
			ip:   "192.168.0.0",
			port: 9090,
			want: "192.168.0.0:9090",
		},
		{
			name: "ipv6 no svc",
			grpc: "2001:db8:1f70::999:de8:7648:6e8:5000",
			ip:   "192.168.0.0",
			port: 9090,
			want: "192.168.0.0:9090",
		},
		{
			name: "ipv6 web svc",
			grpc: "2001:db8:1f70::999:de8:7648:6e8:5000/web",
			ip:   "192.168.0.0",
			port: 9090,
			want: "192.168.0.0:9090/web",
		},
		{
			name: "zone ipv6 web svc",
			grpc: "::FFFF:C0A8:1%1:5000/web",
			ip:   "192.168.0.0",
			port: 9090,
			want: "192.168.0.0:9090/web",
		},
		{
			name: "ipv6 literal web svc",
			grpc: "::FFFF:192.168.0.1:5000/web",
			ip:   "192.168.0.0",
			port: 9090,
			want: "192.168.0.0:9090/web",
		},
		{
			name: "ipv6 injected into ipv6 url",
			grpc: "2001:db8:1f70::999:de8:7648:6e8:5000",
			ip:   "::FFFF:C0A8:1",
			port: 9090,
			want: "::FFFF:C0A8:1:9090",
		},
		{
			name: "ipv6 injected into ipv6 url with svc",
			grpc: "2001:db8:1f70::999:de8:7648:6e8:5000/web",
			ip:   "::FFFF:C0A8:1",
			port: 9090,
			want: "::FFFF:C0A8:1:9090/web",
		},
		{
			name: "ipv6 injected into ipv6 url with special",
			grpc: "2001:db8:1f70::999:de8:7648:6e8:5000/service-$name:with@special:Chars",
			ip:   "::FFFF:C0A8:1",
			port: 9090,
			want: "::FFFF:C0A8:1:9090/service-$name:with@special:Chars",
		},
	}
	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			got := grpcInjectAddr(tt.grpc, tt.ip, tt.port)
			if got != tt.want {
				t.Errorf("httpInjectAddr() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgent_httpInjectAddr(t *testing.T) {
	tt := []struct {
		name string
		url  string
		ip   string
		port int
		want string
	}{
		{
			name: "localhost health",
			url:  "http://localhost:8080/health",
			ip:   "192.168.0.0",
			port: 9090,
			want: "http://192.168.0.0:9090/health",
		},
		{
			name: "https localhost health",
			url:  "https://localhost:8080/health",
			ip:   "192.168.0.0",
			port: 9090,
			want: "https://192.168.0.0:9090/health",
		},
		{
			name: "https ipv4 health",
			url:  "https://127.0.0.1:8080/health",
			ip:   "192.168.0.0",
			port: 9090,
			want: "https://192.168.0.0:9090/health",
		},
		{
			name: "https ipv4 without path",
			url:  "https://127.0.0.1:8080",
			ip:   "192.168.0.0",
			port: 9090,
			want: "https://192.168.0.0:9090",
		},
		{
			name: "https ipv6 health",
			url:  "https://[2001:db8:1f70::999:de8:7648:6e8]:5000/health",
			ip:   "192.168.0.0",
			port: 9090,
			want: "https://192.168.0.0:9090/health",
		},
		{
			name: "https ipv6 with zone",
			url:  "https://[::FFFF:C0A8:1%1]:5000/health",
			ip:   "192.168.0.0",
			port: 9090,
			want: "https://192.168.0.0:9090/health",
		},
		{
			name: "https ipv6 literal",
			url:  "https://[::FFFF:192.168.0.1]:5000/health",
			ip:   "192.168.0.0",
			port: 9090,
			want: "https://192.168.0.0:9090/health",
		},
		{
			name: "https ipv6 without path",
			url:  "https://[2001:db8:1f70::999:de8:7648:6e8]:5000",
			ip:   "192.168.0.0",
			port: 9090,
			want: "https://192.168.0.0:9090",
		},
		{
			name: "ipv6 injected into ipv6 url",
			url:  "https://[2001:db8:1f70::999:de8:7648:6e8]:5000",
			ip:   "::FFFF:C0A8:1",
			port: 9090,
			want: "https://[::FFFF:C0A8:1]:9090",
		},
		{
			name: "ipv6 with brackets injected into ipv6 url",
			url:  "https://[2001:db8:1f70::999:de8:7648:6e8]:5000",
			ip:   "[::FFFF:C0A8:1]",
			port: 9090,
			want: "https://[::FFFF:C0A8:1]:9090",
		},
		{
			name: "short domain health",
			url:  "http://i.co:8080/health",
			ip:   "192.168.0.0",
			port: 9090,
			want: "http://192.168.0.0:9090/health",
		},
		{
			name: "nested url in query",
			url:  "http://my.corp.com:8080/health?from=http://google.com:8080",
			ip:   "192.168.0.0",
			port: 9090,
			want: "http://192.168.0.0:9090/health?from=http://google.com:8080",
		},
	}
	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			got := httpInjectAddr(tt.url, tt.ip, tt.port)
			if got != tt.want {
				t.Errorf("httpInjectAddr() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultIfEmpty(t *testing.T) {
	require.Equal(t, "", defaultIfEmpty("", ""))
	require.Equal(t, "foo", defaultIfEmpty("", "foo"))
	require.Equal(t, "bar", defaultIfEmpty("bar", "foo"))
	require.Equal(t, "bar", defaultIfEmpty("bar", ""))
}

func TestConfigSourceFromName(t *testing.T) {
	cases := []struct {
		in     string
		expect configSource
		bad    bool
	}{
		{in: "local", expect: ConfigSourceLocal},
		{in: "remote", expect: ConfigSourceRemote},
		{in: "", expect: ConfigSourceLocal},
		{in: "LOCAL", bad: true},
		{in: "REMOTE", bad: true},
		{in: "garbage", bad: true},
		{in: " ", bad: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got, ok := ConfigSourceFromName(tc.in)
			if tc.bad {
				require.False(t, ok)
				require.Empty(t, got)
			} else {
				require.True(t, ok)
				require.Equal(t, tc.expect, got)
			}
		})
	}
}

func TestAgent_RerouteExistingHTTPChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register a service without a ProxyAddr
	svc := &structs.NodeService{
		ID:      "web",
		Service: "web",
		Address: "localhost",
		Port:    8080,
	}
	chks := []*structs.CheckType{
		{
			CheckID:       "http",
			HTTP:          "http://localhost:8080/mypath?query",
			Interval:      20 * time.Millisecond,
			TLSSkipVerify: true,
		},
		{
			CheckID:       "grpc",
			GRPC:          "localhost:8080/myservice",
			Interval:      20 * time.Millisecond,
			TLSSkipVerify: true,
		},
	}
	if err := a.addServiceFromSource(svc, chks, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add svc: %v", err)
	}

	// Register a proxy and expose HTTP checks.
	// This should trigger setting ProxyHTTP and ProxyGRPC in the checks.
	proxy := &structs.NodeService{
		Kind:    "connect-proxy",
		ID:      "web-proxy",
		Service: "web-proxy",
		Address: "localhost",
		Port:    21500,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web",
			LocalServiceAddress:    "localhost",
			LocalServicePort:       8080,
			MeshGateway:            structs.MeshGatewayConfig{},
			Expose: structs.ExposeConfig{
				Checks: true,
			},
		},
	}
	if err := a.addServiceFromSource(proxy, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add svc: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		chks := a.ServiceHTTPBasedChecks(structs.NewServiceID("web", nil))
		require.Equal(r, chks[0].ProxyHTTP, "http://localhost:21500/mypath?query")
	})

	retry.Run(t, func(r *retry.R) {
		hc := a.State.Check(structs.NewCheckID("http", nil))
		require.Equal(r, hc.ExposedPort, 21500)
	})

	retry.Run(t, func(r *retry.R) {
		chks := a.ServiceHTTPBasedChecks(structs.NewServiceID("web", nil))

		// GRPC check will be at a later index than HTTP check because of the fetching order in ServiceHTTPBasedChecks.
		// Note that this relies on listener ports auto-incrementing in a.listenerPortLocked.
		require.Equal(r, chks[1].ProxyGRPC, "localhost:21501/myservice")
	})

	retry.Run(t, func(r *retry.R) {
		hc := a.State.Check(structs.NewCheckID("grpc", nil))
		require.Equal(r, hc.ExposedPort, 21501)
	})

	// Re-register a proxy and disable exposing HTTP checks.
	// This should trigger resetting ProxyHTTP and ProxyGRPC to empty strings
	// and reset saved exposed ports in the agent's state.
	proxy = &structs.NodeService{
		Kind:    "connect-proxy",
		ID:      "web-proxy",
		Service: "web-proxy",
		Address: "localhost",
		Port:    21500,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web",
			LocalServiceAddress:    "localhost",
			LocalServicePort:       8080,
			MeshGateway:            structs.MeshGatewayConfig{},
			Expose: structs.ExposeConfig{
				Checks: false,
			},
		},
	}
	if err := a.addServiceFromSource(proxy, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add svc: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		chks := a.ServiceHTTPBasedChecks(structs.NewServiceID("web", nil))
		require.Empty(r, chks[0].ProxyHTTP, "ProxyHTTP addr was not reset")
	})

	retry.Run(t, func(r *retry.R) {
		hc := a.State.Check(structs.NewCheckID("http", nil))
		require.Equal(r, hc.ExposedPort, 0)
	})

	retry.Run(t, func(r *retry.R) {
		chks := a.ServiceHTTPBasedChecks(structs.NewServiceID("web", nil))

		// Will be at a later index than HTTP check because of the fetching order in ServiceHTTPBasedChecks.
		require.Empty(r, chks[1].ProxyGRPC, "ProxyGRPC addr was not reset")
	})

	retry.Run(t, func(r *retry.R) {
		hc := a.State.Check(structs.NewCheckID("grpc", nil))
		require.Equal(r, hc.ExposedPort, 0)
	})
}

func TestAgent_RerouteNewHTTPChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register a service without a ProxyAddr
	svc := &structs.NodeService{
		ID:      "web",
		Service: "web",
		Address: "localhost",
		Port:    8080,
	}
	if err := a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add svc: %v", err)
	}

	// Register a proxy and expose HTTP checks
	proxy := &structs.NodeService{
		Kind:    "connect-proxy",
		ID:      "web-proxy",
		Service: "web-proxy",
		Address: "localhost",
		Port:    21500,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web",
			LocalServiceAddress:    "localhost",
			LocalServicePort:       8080,
			MeshGateway:            structs.MeshGatewayConfig{},
			Expose: structs.ExposeConfig{
				Checks: true,
			},
		},
	}
	if err := a.addServiceFromSource(proxy, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add svc: %v", err)
	}

	checks := []*structs.HealthCheck{
		{
			CheckID:   "http",
			Name:      "http",
			ServiceID: "web",
			Status:    api.HealthCritical,
		},
		{
			CheckID:   "grpc",
			Name:      "grpc",
			ServiceID: "web",
			Status:    api.HealthCritical,
		},
	}
	chkTypes := []*structs.CheckType{
		{
			CheckID:       "http",
			HTTP:          "http://localhost:8080/mypath?query",
			Interval:      20 * time.Millisecond,
			TLSSkipVerify: true,
		},
		{
			CheckID:       "grpc",
			GRPC:          "localhost:8080/myservice",
			Interval:      20 * time.Millisecond,
			TLSSkipVerify: true,
		},
	}

	// ProxyGRPC and ProxyHTTP should be set when creating check
	// since proxy.expose.checks is enabled on the proxy
	if err := a.AddCheck(checks[0], chkTypes[0], false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add check: %v", err)
	}
	if err := a.AddCheck(checks[1], chkTypes[1], false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add check: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		chks := a.ServiceHTTPBasedChecks(structs.NewServiceID("web", nil))
		require.Equal(r, chks[0].ProxyHTTP, "http://localhost:21500/mypath?query")
	})

	retry.Run(t, func(r *retry.R) {
		hc := a.State.Check(structs.NewCheckID("http", nil))
		require.Equal(r, hc.ExposedPort, 21500)
	})

	retry.Run(t, func(r *retry.R) {
		chks := a.ServiceHTTPBasedChecks(structs.NewServiceID("web", nil))

		// GRPC check will be at a later index than HTTP check because of the fetching order in ServiceHTTPBasedChecks.
		require.Equal(r, chks[1].ProxyGRPC, "localhost:21501/myservice")
	})

	retry.Run(t, func(r *retry.R) {
		hc := a.State.Check(structs.NewCheckID("grpc", nil))
		require.Equal(r, hc.ExposedPort, 21501)
	})
}

func TestAgentCache_serviceInConfigFile_initialFetchErrors_Issue6521(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Ensure that initial failures to fetch the discovery chain via the agent
	// cache using the notify API for a service with no config entries
	// correctly recovers when those RPCs resume working. The key here is that
	// the lack of config entries guarantees that the RPC will come back with a
	// synthetic index of 1.
	//
	// The bug in the Cache.notifyBlockingQuery used to incorrectly "fix" the
	// index for the next query from 0 to 1 for all queries, when it should
	// have not done so for queries that errored.

	a1 := StartTestAgent(t, TestAgent{Name: "Agent1"})
	defer a1.Shutdown()
	testrpc.WaitForLeader(t, a1.RPC, "dc1")

	a2 := StartTestAgent(t, TestAgent{Name: "Agent2", HCL: `
		server = false
		bootstrap = false
services {
  name = "echo-client"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams {
          destination_name = "echo"
          local_bind_port  = 9191
        }
      }
    }
  }
}

services {
  name = "echo"
  port = 9090
  connect {
    sidecar_service {}
  }
}
	`})
	defer a2.Shutdown()

	// Starting a client agent disconnected from a server with services.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan cache.UpdateEvent, 1)
	require.NoError(t, a2.cache.Notify(ctx, cachetype.CompiledDiscoveryChainName, &structs.DiscoveryChainRequest{
		Datacenter:           "dc1",
		Name:                 "echo",
		EvaluateInDatacenter: "dc1",
		EvaluateInNamespace:  "default",
	}, "foo", ch))

	{ // The first event is an error because we are not joined yet.
		evt := <-ch
		require.Equal(t, "foo", evt.CorrelationID)
		require.Nil(t, evt.Result)
		require.Error(t, evt.Err)
		require.Equal(t, evt.Err, structs.ErrNoServers)
	}

	t.Logf("joining client to server")

	// Now connect to server
	_, err := a1.JoinLAN([]string{
		fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN),
	}, nil)
	require.NoError(t, err)

	t.Logf("joined client to server")

	deadlineCh := time.After(10 * time.Second)
	start := time.Now()
LOOP:
	for {
		select {
		case evt := <-ch:
			// We may receive several notifications of an error until we get the
			// first successful reply.
			require.Equal(t, "foo", evt.CorrelationID)
			if evt.Err != nil {
				break LOOP
			}
			require.NoError(t, evt.Err)
			require.NotNil(t, evt.Result)
			t.Logf("took %s to get first success", time.Since(start))
		case <-deadlineCh:
			t.Fatal("did not get notified successfully")
		}
	}
}

// This is a mirror of a similar test in agent/consul/server_test.go
//
// TODO(rb): implement something similar to this as a full containerized test suite with proper
// isolation so requests can't "cheat" and bypass the mesh gateways
func TestAgent_JoinWAN_viaMeshGateway(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	port := freeport.GetOne(t)
	gwAddr := ipaddr.FormatAddressPort("127.0.0.1", port)

	// Due to some ordering, we'll have to manually configure these ports in
	// advance.
	secondaryRPCPorts := freeport.GetN(t, 2)

	a1 := StartTestAgent(t, TestAgent{Name: "bob", HCL: `
		domain = "consul"
		node_name = "bob"
		datacenter = "dc1"
		primary_datacenter = "dc1"
		# tls
		ca_file = "../test/hostname/CertAuth.crt"
		cert_file = "../test/hostname/Bob.crt"
		key_file = "../test/hostname/Bob.key"
		verify_incoming               = true
		verify_outgoing               = true
		verify_server_hostname        = true
		# wanfed
		connect {
			enabled = true
			enable_mesh_gateway_wan_federation = true
		}
	`})
	defer a1.Shutdown()
	testrpc.WaitForTestAgent(t, a1.RPC, "dc1")

	// We'll use the same gateway for all datacenters since it doesn't care.
	var (
		rpcAddr1 = ipaddr.FormatAddressPort("127.0.0.1", a1.Config.ServerPort)
		rpcAddr2 = ipaddr.FormatAddressPort("127.0.0.1", secondaryRPCPorts[0])
		rpcAddr3 = ipaddr.FormatAddressPort("127.0.0.1", secondaryRPCPorts[1])
	)
	var p tcpproxy.Proxy
	p.AddSNIRoute(gwAddr, "bob.server.dc1.consul", tcpproxy.To(rpcAddr1))
	p.AddSNIRoute(gwAddr, "server.dc1.consul", tcpproxy.To(rpcAddr1))
	p.AddSNIRoute(gwAddr, "betty.server.dc2.consul", tcpproxy.To(rpcAddr2))
	p.AddSNIRoute(gwAddr, "server.dc2.consul", tcpproxy.To(rpcAddr2))
	p.AddSNIRoute(gwAddr, "bonnie.server.dc3.consul", tcpproxy.To(rpcAddr3))
	p.AddSNIRoute(gwAddr, "server.dc3.consul", tcpproxy.To(rpcAddr3))
	p.AddStopACMESearch(gwAddr)
	require.NoError(t, p.Start())
	defer func() {
		p.Close()
		p.Wait()
	}()

	t.Logf("routing %s => %s", "{bob.,}server.dc1.consul", rpcAddr1)
	t.Logf("routing %s => %s", "{betty.,}server.dc2.consul", rpcAddr2)
	t.Logf("routing %s => %s", "{bonnie.,}server.dc3.consul", rpcAddr3)

	// Register this into the agent in dc1.
	{
		args := &structs.ServiceDefinition{
			Kind: structs.ServiceKindMeshGateway,
			ID:   "mesh-gateway",
			Name: "mesh-gateway",
			Meta: map[string]string{structs.MetaWANFederationKey: "1"},
			Port: port,
		}
		req, err := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		require.NoError(t, err)

		obj, err := a1.srv.AgentRegisterService(nil, req)
		require.NoError(t, err)
		require.Nil(t, obj)
	}

	waitForFederationState := func(t *testing.T, a *TestAgent, dc string) {
		retry.Run(t, func(r *retry.R) {
			req, err := http.NewRequest("GET", "/v1/internal/federation-state/"+dc, nil)
			require.NoError(r, err)

			resp := httptest.NewRecorder()
			obj, err := a.srv.FederationStateGet(resp, req)
			require.NoError(r, err)
			require.NotNil(r, obj)

			out, ok := obj.(structs.FederationStateResponse)
			require.True(r, ok)
			require.NotNil(r, out.State)
			require.Len(r, out.State.MeshGateways, 1)
		})
	}

	// Wait until at least catalog AE and federation state AE fire.
	waitForFederationState(t, a1, "dc1")
	retry.Run(t, func(r *retry.R) {
		require.NotEmpty(r, a1.PickRandomMeshGatewaySuitableForDialing("dc1"))
	})

	a2 := StartTestAgent(t, TestAgent{Name: "betty", HCL: `
		domain = "consul"
		node_name = "betty"
		datacenter = "dc2"
		primary_datacenter = "dc1"
		# tls
		ca_file = "../test/hostname/CertAuth.crt"
		cert_file = "../test/hostname/Betty.crt"
		key_file = "../test/hostname/Betty.key"
		verify_incoming               = true
		verify_outgoing               = true
		verify_server_hostname        = true
		ports {
			server = ` + strconv.Itoa(secondaryRPCPorts[0]) + `
		}
		# wanfed
		primary_gateways = ["` + gwAddr + `"]
		retry_interval_wan = "250ms"
		connect {
			enabled = true
			enable_mesh_gateway_wan_federation = true
		}
	`})
	defer a2.Shutdown()
	testrpc.WaitForTestAgent(t, a2.RPC, "dc2")

	a3 := StartTestAgent(t, TestAgent{Name: "bonnie", HCL: `
		domain = "consul"
		node_name = "bonnie"
		datacenter = "dc3"
		primary_datacenter = "dc1"
		# tls
		ca_file = "../test/hostname/CertAuth.crt"
		cert_file = "../test/hostname/Bonnie.crt"
		key_file = "../test/hostname/Bonnie.key"
		verify_incoming               = true
		verify_outgoing               = true
		verify_server_hostname        = true
		ports {
			server = ` + strconv.Itoa(secondaryRPCPorts[1]) + `
		}
		# wanfed
		primary_gateways = ["` + gwAddr + `"]
		retry_interval_wan = "250ms"
		connect {
			enabled = true
			enable_mesh_gateway_wan_federation = true
		}
	`})
	defer a3.Shutdown()
	testrpc.WaitForTestAgent(t, a3.RPC, "dc3")

	// The primary_gateways config setting should cause automatic mesh join.
	// Assert that the secondaries have joined the primary.
	findPrimary := func(r *retry.R, a *TestAgent) *serf.Member {
		var primary *serf.Member
		for _, m := range a.WANMembers() {
			if m.Tags["dc"] == "dc1" {
				require.Nil(r, primary, "already found one node in dc1")
				primary = &m
			}
		}
		require.NotNil(r, primary)
		return primary
	}
	retry.Run(t, func(r *retry.R) {
		p2, p3 := findPrimary(r, a2), findPrimary(r, a3)
		require.Equal(r, "bob.dc1", p2.Name)
		require.Equal(r, "bob.dc1", p3.Name)
	})

	testrpc.WaitForLeader(t, a2.RPC, "dc2")
	testrpc.WaitForLeader(t, a3.RPC, "dc3")

	// Now we can register this into the catalog in dc2 and dc3.
	{
		args := &structs.ServiceDefinition{
			Kind: structs.ServiceKindMeshGateway,
			ID:   "mesh-gateway",
			Name: "mesh-gateway",
			Meta: map[string]string{structs.MetaWANFederationKey: "1"},
			Port: port,
		}
		req, err := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		require.NoError(t, err)

		obj, err := a2.srv.AgentRegisterService(nil, req)
		require.NoError(t, err)
		require.Nil(t, obj)
	}
	{
		args := &structs.ServiceDefinition{
			Kind: structs.ServiceKindMeshGateway,
			ID:   "mesh-gateway",
			Name: "mesh-gateway",
			Meta: map[string]string{structs.MetaWANFederationKey: "1"},
			Port: port,
		}
		req, err := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		require.NoError(t, err)

		obj, err := a3.srv.AgentRegisterService(nil, req)
		require.NoError(t, err)
		require.Nil(t, obj)
	}

	// Wait until federation state replication functions
	waitForFederationState(t, a1, "dc1")
	waitForFederationState(t, a1, "dc2")
	waitForFederationState(t, a1, "dc3")

	waitForFederationState(t, a2, "dc1")
	waitForFederationState(t, a2, "dc2")
	waitForFederationState(t, a2, "dc3")

	waitForFederationState(t, a3, "dc1")
	waitForFederationState(t, a3, "dc2")
	waitForFederationState(t, a3, "dc3")

	retry.Run(t, func(r *retry.R) {
		require.NotEmpty(r, a1.PickRandomMeshGatewaySuitableForDialing("dc1"))
		require.NotEmpty(r, a1.PickRandomMeshGatewaySuitableForDialing("dc2"))
		require.NotEmpty(r, a1.PickRandomMeshGatewaySuitableForDialing("dc3"))

		require.NotEmpty(r, a2.PickRandomMeshGatewaySuitableForDialing("dc1"))
		require.NotEmpty(r, a2.PickRandomMeshGatewaySuitableForDialing("dc2"))
		require.NotEmpty(r, a2.PickRandomMeshGatewaySuitableForDialing("dc3"))

		require.NotEmpty(r, a3.PickRandomMeshGatewaySuitableForDialing("dc1"))
		require.NotEmpty(r, a3.PickRandomMeshGatewaySuitableForDialing("dc2"))
		require.NotEmpty(r, a3.PickRandomMeshGatewaySuitableForDialing("dc3"))
	})

	retry.Run(t, func(r *retry.R) {
		if got, want := len(a1.WANMembers()), 3; got != want {
			r.Fatalf("got %d WAN members want at least %d", got, want)
		}
		if got, want := len(a2.WANMembers()), 3; got != want {
			r.Fatalf("got %d WAN members want at least %d", got, want)
		}
		if got, want := len(a3.WANMembers()), 3; got != want {
			r.Fatalf("got %d WAN members want at least %d", got, want)
		}
	})

	// Ensure we can do some trivial RPC in all directions.
	//
	// NOTE: we explicitly make streaming and non-streaming assertions here to
	// verify both rpc and grpc codepaths.
	agents := map[string]*TestAgent{"dc1": a1, "dc2": a2, "dc3": a3}
	names := map[string]string{"dc1": "bob", "dc2": "betty", "dc3": "bonnie"}
	for _, srcDC := range []string{"dc1", "dc2", "dc3"} {
		a := agents[srcDC]
		for _, dstDC := range []string{"dc1", "dc2", "dc3"} {
			if srcDC == dstDC {
				continue
			}
			t.Run(srcDC+" to "+dstDC, func(t *testing.T) {
				t.Run("normal-rpc", func(t *testing.T) {
					req, err := http.NewRequest("GET", "/v1/catalog/nodes?dc="+dstDC, nil)
					require.NoError(t, err)

					resp := httptest.NewRecorder()
					obj, err := a.srv.CatalogNodes(resp, req)
					require.NoError(t, err)
					require.NotNil(t, obj)

					nodes, ok := obj.(structs.Nodes)
					require.True(t, ok)
					require.Len(t, nodes, 1)
					node := nodes[0]
					require.Equal(t, dstDC, node.Datacenter)
					require.Equal(t, names[dstDC], node.Node)
				})
				t.Run("streaming-grpc", func(t *testing.T) {
					req, err := http.NewRequest("GET", "/v1/health/service/consul?cached&dc="+dstDC, nil)
					require.NoError(t, err)

					resp := httptest.NewRecorder()
					obj, err := a.srv.HealthServiceNodes(resp, req)
					require.NoError(t, err)
					require.NotNil(t, obj)

					csns, ok := obj.(structs.CheckServiceNodes)
					require.True(t, ok)
					require.Len(t, csns, 1)

					csn := csns[0]
					require.Equal(t, dstDC, csn.Node.Datacenter)
					require.Equal(t, names[dstDC], csn.Node.Node)
				})
			})
		}
	}
}

func TestAutoConfig_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// eventually this test should really live with integration tests
	// the goal here is to have one test server and another test client
	// spin up both agents and allow the server to authorize the auto config
	// request and then see the client joined. Finally we force a CA roots
	// update and wait to see that the agents TLS certificate gets updated.

	cfgDir := testutil.TempDir(t, "auto-config")

	// write some test TLS certificates out to the cfg dir
	cert, key, cacert, err := testTLSCertificates("server.dc1.consul")
	require.NoError(t, err)

	certFile := filepath.Join(cfgDir, "cert.pem")
	caFile := filepath.Join(cfgDir, "cacert.pem")
	keyFile := filepath.Join(cfgDir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, []byte(cert), 0600))
	require.NoError(t, os.WriteFile(caFile, []byte(cacert), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte(key), 0600))

	// generate a gossip key
	gossipKey := make([]byte, 32)
	n, err := rand.Read(gossipKey)
	require.NoError(t, err)
	require.Equal(t, 32, n)
	gossipKeyEncoded := base64.StdEncoding.EncodeToString(gossipKey)

	// generate the JWT signing keys
	pub, priv, err := oidcauthtest.GenerateKey()
	require.NoError(t, err)

	hclConfig := TestACLConfigWithParams(nil) + `
		encrypt = "` + gossipKeyEncoded + `"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "` + caFile + `"
		cert_file = "` + certFile + `"
		key_file = "` + keyFile + `"
		connect { enabled = true }
		auto_config {
			authorization {
				enabled = true
				static {
					claim_mappings = {
						consul_node_name = "node"
					}
					claim_assertions = [
						"value.node == \"${node}\""
					]
					bound_issuer = "consul"
					bound_audiences = [
						"consul"
					]
					jwt_validation_pub_keys = ["` + strings.ReplaceAll(pub, "\n", "\\n") + `"]
				}
			}
		}
	`

	srv := StartTestAgent(t, TestAgent{Name: "TestAgent-Server", HCL: hclConfig})
	defer srv.Shutdown()

	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	// sign a JWT token
	now := time.Now()
	token, err := oidcauthtest.SignJWT(priv, jwt.Claims{
		Subject:   "consul",
		Issuer:    "consul",
		Audience:  jwt.Audience{"consul"},
		NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Second)),
		Expiry:    jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}, map[string]interface{}{
		"consul_node_name": "test-client",
	})
	require.NoError(t, err)

	client := StartTestAgent(t, TestAgent{Name: "test-client",
		Overrides: `
			connect {
				test_ca_leaf_root_change_spread = "1ns"
			}
		`,
		HCL: `
			bootstrap = false
			server = false
			ca_file = "` + caFile + `"
			verify_outgoing = true
			verify_server_hostname = true
			node_name = "test-client"
			ports {
				server = ` + strconv.Itoa(srv.Config.RPCBindAddr.Port) + `
			}
			auto_config {
				enabled = true
				intro_token = "` + token + `"
				server_addresses = ["` + srv.Config.RPCBindAddr.String() + `"]
			}`,
	})

	defer client.Shutdown()

	retry.Run(t, func(r *retry.R) {
		require.NotNil(r, client.Agent.tlsConfigurator.Cert())
	})

	// when this is successful we managed to get the gossip key and serf addresses to bind to
	// and then connect. Additionally we would have to have certificates or else the
	// verify_incoming config on the server would not let it work.
	testrpc.WaitForTestAgent(t, client.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	// spot check that we now have an ACL token
	require.NotEmpty(t, client.tokens.AgentToken())

	// grab the existing cert
	cert1 := client.Agent.tlsConfigurator.Cert()
	require.NotNil(t, cert1)

	// force a roots rotation by updating the CA config
	t.Logf("Forcing roots rotation on the server")
	ca := connect.TestCA(t, nil)
	req := &structs.CARequest{
		Datacenter:   "dc1",
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		Config: &structs.CAConfiguration{
			Provider: "consul",
			Config: map[string]interface{}{
				"LeafCertTTL":         "1h",
				"PrivateKey":          ca.SigningKey,
				"RootCert":            ca.RootCert,
				"IntermediateCertTTL": "3h",
			},
		},
	}
	var reply interface{}
	require.NoError(t, srv.RPC("ConnectCA.ConfigurationSet", &req, &reply))

	// ensure that a new cert gets generated and pushed into the TLS configurator
	retry.Run(t, func(r *retry.R) {
		require.NotEqual(r, cert1, client.Agent.tlsConfigurator.Cert())

		// check that the on disk certs match expectations
		data, err := os.ReadFile(filepath.Join(client.DataDir, "auto-config.json"))
		require.NoError(r, err)
		rdr := strings.NewReader(string(data))

		var resp pbautoconf.AutoConfigResponse
		pbUnmarshaler := &jsonpb.Unmarshaler{
			AllowUnknownFields: false,
		}
		require.NoError(r, pbUnmarshaler.Unmarshal(rdr, &resp), "data: %s", data)

		actual, err := tls.X509KeyPair([]byte(resp.Certificate.CertPEM), []byte(resp.Certificate.PrivateKeyPEM))
		require.NoError(r, err)
		require.Equal(r, client.Agent.tlsConfigurator.Cert(), &actual)
	})
}

func TestAgent_AutoEncrypt(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// eventually this test should really live with integration tests
	// the goal here is to have one test server and another test client
	// spin up both agents and allow the server to authorize the auto encrypt
	// request and then see the client get a TLS certificate
	cfgDir := testutil.TempDir(t, "auto-encrypt")

	// write some test TLS certificates out to the cfg dir
	cert, key, cacert, err := testTLSCertificates("server.dc1.consul")
	require.NoError(t, err)

	certFile := filepath.Join(cfgDir, "cert.pem")
	caFile := filepath.Join(cfgDir, "cacert.pem")
	keyFile := filepath.Join(cfgDir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, []byte(cert), 0600))
	require.NoError(t, os.WriteFile(caFile, []byte(cacert), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte(key), 0600))

	hclConfig := TestACLConfigWithParams(nil) + `
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "` + caFile + `"
		cert_file = "` + certFile + `"
		key_file = "` + keyFile + `"
		connect { enabled = true }
		auto_encrypt { allow_tls = true }
	`

	srv := StartTestAgent(t, TestAgent{Name: "test-server", HCL: hclConfig})
	defer srv.Shutdown()

	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	client := StartTestAgent(t, TestAgent{Name: "test-client", HCL: TestACLConfigWithParams(nil) + `
	   bootstrap = false
		server = false
		ca_file = "` + caFile + `"
		verify_outgoing = true
		verify_server_hostname = true
		node_name = "test-client"
		auto_encrypt {
			tls = true
		}
		ports {
			server = ` + strconv.Itoa(srv.Config.RPCBindAddr.Port) + `
		}
		retry_join = ["` + srv.Config.SerfBindAddrLAN.String() + `"]`,
		UseTLS: true,
	})

	defer client.Shutdown()

	// when this is successful we managed to get a TLS certificate and are using it for
	// encrypted RPC connections.
	testrpc.WaitForTestAgent(t, client.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	// now we need to validate that our certificate has the correct CN
	aeCert := client.tlsConfigurator.Cert()
	require.NotNil(t, aeCert)

	id := connect.SpiffeIDAgent{
		Host:       connect.TestClusterID + ".consul",
		Datacenter: "dc1",
		Agent:      "test-client",
	}
	x509Cert, err := x509.ParseCertificate(aeCert.Certificate[0])
	require.NoError(t, err)
	require.Empty(t, x509Cert.Subject.CommonName)
	require.Len(t, x509Cert.URIs, 1)
	require.Equal(t, id.URI(), x509Cert.URIs[0])
}

func TestSharedRPCRouter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// this test runs both a server and client and ensures that the shared
	// router is being used. It would be possible for the Client and Server
	// types to create and use their own routers and for RPCs such as the
	// ones used in WaitForTestAgent to succeed. However accessing the
	// router stored on the agent ensures that Serf information from the
	// Client/Server types are being set in the same shared rpc router.

	srv := NewTestAgent(t, "")
	defer srv.Shutdown()

	testrpc.WaitForTestAgent(t, srv.RPC, "dc1")

	mgr, server := srv.Agent.baseDeps.Router.FindLANRoute()
	require.NotNil(t, mgr)
	require.NotNil(t, server)

	client := NewTestAgent(t, `
		server = false
		bootstrap = false
		retry_join = ["`+srv.Config.SerfBindAddrLAN.String()+`"]
	`)

	testrpc.WaitForTestAgent(t, client.RPC, "dc1")

	mgr, server = client.Agent.baseDeps.Router.FindLANRoute()
	require.NotNil(t, mgr)
	require.NotNil(t, server)
}

func TestAgent_ListenHTTP_MultipleAddresses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ports := freeport.GetN(t, 2)
	caConfig := tlsutil.Config{}
	tlsConf, err := tlsutil.NewConfigurator(caConfig, hclog.New(nil))
	require.NoError(t, err)
	bd := BaseDeps{
		Deps: consul.Deps{
			Logger:          hclog.NewInterceptLogger(nil),
			Tokens:          new(token.Store),
			TLSConfigurator: tlsConf,
			GRPCConnPool:    &fakeGRPCConnPool{},
		},
		RuntimeConfig: &config.RuntimeConfig{
			HTTPAddrs: []net.Addr{
				&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[0]},
				&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[1]},
			},
		},
		Cache: cache.New(cache.Options{}),
	}

	bd, err = initEnterpriseBaseDeps(bd, nil)
	require.NoError(t, err)

	agent, err := New(bd)
	require.NoError(t, err)

	agent.startLicenseManager(testutil.TestContext(t))

	srvs, err := agent.listenHTTP()
	require.NoError(t, err)
	defer func() {
		ctx := context.Background()
		for _, srv := range srvs {
			srv.Shutdown(ctx)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	g := new(errgroup.Group)
	for _, s := range srvs {
		g.Go(s.Run)
	}

	require.Len(t, srvs, 2)
	require.Len(t, uniqueAddrs(srvs), 2)

	client := &http.Client{}
	for _, s := range srvs {
		u := url.URL{Scheme: s.Protocol, Host: s.Addr.String()}
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		require.NoError(t, err)

		resp, err := client.Do(req.WithContext(ctx))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
	}
}

func uniqueAddrs(srvs []apiServer) map[string]struct{} {
	result := make(map[string]struct{}, len(srvs))
	for _, s := range srvs {
		result[s.Addr.String()] = struct{}{}
	}
	return result
}

func TestAgent_AutoReloadDoReload_WhenCertAndKeyUpdated(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	certsDir := testutil.TempDir(t, "auto-config")

	// write some test TLS certificates out to the cfg dir
	serverName := "server.dc1.consul"
	signer, _, err := tlsutil.GeneratePrivateKey()
	require.NoError(t, err)

	ca, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	require.NoError(t, err)

	cert, privateKey, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)

	certFile := filepath.Join(certsDir, "cert.pem")
	caFile := filepath.Join(certsDir, "cacert.pem")
	keyFile := filepath.Join(certsDir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, []byte(cert), 0600))
	require.NoError(t, os.WriteFile(caFile, []byte(ca), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte(privateKey), 0600))

	// generate a gossip key
	gossipKey := make([]byte, 32)
	n, err := rand.Read(gossipKey)
	require.NoError(t, err)
	require.Equal(t, 32, n)
	gossipKeyEncoded := base64.StdEncoding.EncodeToString(gossipKey)

	hclConfig := TestACLConfigWithParams(nil) + `
		encrypt = "` + gossipKeyEncoded + `"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "` + caFile + `"
		cert_file = "` + certFile + `"
		key_file = "` + keyFile + `"
		connect { enabled = true }
		auto_reload_config = true
	`

	srv := StartTestAgent(t, TestAgent{Name: "TestAgent-Server", HCL: hclConfig})
	defer srv.Shutdown()

	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	aeCert := srv.tlsConfigurator.Cert()
	require.NotNil(t, aeCert)

	cert2, privateKey2, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certFile, []byte(cert2), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte(privateKey2), 0600))

	retry.Run(t, func(r *retry.R) {
		aeCert2 := srv.tlsConfigurator.Cert()
		require.NotEqual(r, aeCert.Certificate, aeCert2.Certificate)
	})

}

func TestAgent_AutoReloadDoNotReload_WhenCaUpdated(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	certsDir := testutil.TempDir(t, "auto-config")

	// write some test TLS certificates out to the cfg dir
	serverName := "server.dc1.consul"
	signer, _, err := tlsutil.GeneratePrivateKey()
	require.NoError(t, err)

	ca, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	require.NoError(t, err)

	cert, privateKey, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)

	certFile := filepath.Join(certsDir, "cert.pem")
	caFile := filepath.Join(certsDir, "cacert.pem")
	keyFile := filepath.Join(certsDir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, []byte(cert), 0600))
	require.NoError(t, os.WriteFile(caFile, []byte(ca), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte(privateKey), 0600))

	// generate a gossip key
	gossipKey := make([]byte, 32)
	n, err := rand.Read(gossipKey)
	require.NoError(t, err)
	require.Equal(t, 32, n)
	gossipKeyEncoded := base64.StdEncoding.EncodeToString(gossipKey)

	hclConfig := TestACLConfigWithParams(nil) + `
		encrypt = "` + gossipKeyEncoded + `"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "` + caFile + `"
		cert_file = "` + certFile + `"
		key_file = "` + keyFile + `"
		connect { enabled = true }
		auto_reload_config = true
	`

	srv := StartTestAgent(t, TestAgent{Name: "TestAgent-Server", HCL: hclConfig})
	defer srv.Shutdown()

	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	aeCA := srv.tlsConfigurator.ManualCAPems()
	require.NotNil(t, aeCA)

	ca2, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(caFile, []byte(ca2), 0600))

	// wait a bit to see if it get updated.
	time.Sleep(time.Second)

	aeCA2 := srv.tlsConfigurator.ManualCAPems()
	require.NotNil(t, aeCA2)
	require.Equal(t, aeCA, aeCA2)
}

func TestAgent_AutoReloadDoReload_WhenCertThenKeyUpdated(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	certsDir := testutil.TempDir(t, "auto-config")

	// write some test TLS certificates out to the cfg dir
	serverName := "server.dc1.consul"
	signer, _, err := tlsutil.GeneratePrivateKey()
	require.NoError(t, err)

	ca, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	require.NoError(t, err)

	cert, privateKey, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)

	certFile := filepath.Join(certsDir, "cert.pem")
	caFile := filepath.Join(certsDir, "cacert.pem")
	keyFile := filepath.Join(certsDir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, []byte(cert), 0600))
	require.NoError(t, os.WriteFile(caFile, []byte(ca), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte(privateKey), 0600))

	// generate a gossip key
	gossipKey := make([]byte, 32)
	n, err := rand.Read(gossipKey)
	require.NoError(t, err)
	require.Equal(t, 32, n)
	gossipKeyEncoded := base64.StdEncoding.EncodeToString(gossipKey)

	hclConfig := TestACLConfigWithParams(nil)

	configFile := testutil.TempDir(t, "config") + "/config.hcl"
	require.NoError(t, os.WriteFile(configFile, []byte(`
		encrypt = "`+gossipKeyEncoded+`"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "`+caFile+`"
		cert_file = "`+certFile+`"
		key_file = "`+keyFile+`"
		connect { enabled = true }
		auto_reload_config = true
	`), 0600))

	srv := StartTestAgent(t, TestAgent{Name: "TestAgent-Server", HCL: hclConfig, configFiles: []string{configFile}})
	defer srv.Shutdown()

	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	cert1Pub := srv.tlsConfigurator.Cert().Certificate
	cert1Key := srv.tlsConfigurator.Cert().PrivateKey

	certNew, privateKeyNew, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)
	certFileNew := filepath.Join(certsDir, "cert_new.pem")
	require.NoError(t, os.WriteFile(certFileNew, []byte(certNew), 0600))
	require.NoError(t, os.WriteFile(configFile, []byte(`
		encrypt = "`+gossipKeyEncoded+`"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "`+caFile+`"
		cert_file = "`+certFileNew+`"
		key_file = "`+keyFile+`"
		connect { enabled = true }
		auto_reload_config = true
	`), 0600))

	// cert should not change as we did not update the associated key
	time.Sleep(1 * time.Second)
	retry.Run(t, func(r *retry.R) {
		cert := srv.tlsConfigurator.Cert()
		require.NotNil(r, cert)
		require.Equal(r, cert1Pub, cert.Certificate)
		require.Equal(r, cert1Key, cert.PrivateKey)
	})

	require.NoError(t, os.WriteFile(keyFile, []byte(privateKeyNew), 0600))

	// cert should change as we did not update the associated key
	time.Sleep(1 * time.Second)
	retry.Run(t, func(r *retry.R) {
		require.NotEqual(r, cert1Pub, srv.tlsConfigurator.Cert().Certificate)
		require.NotEqual(r, cert1Key, srv.tlsConfigurator.Cert().PrivateKey)
	})
}

func TestAgent_AutoReloadDoReload_WhenKeyThenCertUpdated(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	certsDir := testutil.TempDir(t, "auto-config")

	// write some test TLS certificates out to the cfg dir
	serverName := "server.dc1.consul"
	signer, _, err := tlsutil.GeneratePrivateKey()
	require.NoError(t, err)

	ca, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	require.NoError(t, err)

	cert, privateKey, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)

	certFile := filepath.Join(certsDir, "cert.pem")
	caFile := filepath.Join(certsDir, "cacert.pem")
	keyFile := filepath.Join(certsDir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, []byte(cert), 0600))
	require.NoError(t, os.WriteFile(caFile, []byte(ca), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte(privateKey), 0600))

	// generate a gossip key
	gossipKey := make([]byte, 32)
	n, err := rand.Read(gossipKey)
	require.NoError(t, err)
	require.Equal(t, 32, n)
	gossipKeyEncoded := base64.StdEncoding.EncodeToString(gossipKey)

	hclConfig := TestACLConfigWithParams(nil)

	configFile := testutil.TempDir(t, "config") + "/config.hcl"
	require.NoError(t, os.WriteFile(configFile, []byte(`
		encrypt = "`+gossipKeyEncoded+`"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "`+caFile+`"
		cert_file = "`+certFile+`"
		key_file = "`+keyFile+`"
		connect { enabled = true }
		auto_reload_config = true
	`), 0600))

	srv := StartTestAgent(t, TestAgent{Name: "TestAgent-Server", HCL: hclConfig, configFiles: []string{configFile}})

	defer srv.Shutdown()

	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	cert1Pub := srv.tlsConfigurator.Cert().Certificate
	cert1Key := srv.tlsConfigurator.Cert().PrivateKey

	certNew, privateKeyNew, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)
	certFileNew := filepath.Join(certsDir, "cert_new.pem")
	require.NoError(t, os.WriteFile(keyFile, []byte(privateKeyNew), 0600))
	// cert should not change as we did not update the associated key
	time.Sleep(1 * time.Second)
	retry.Run(t, func(r *retry.R) {
		cert := srv.tlsConfigurator.Cert()
		require.NotNil(r, cert)
		require.Equal(r, cert1Pub, cert.Certificate)
		require.Equal(r, cert1Key, cert.PrivateKey)
	})

	require.NoError(t, os.WriteFile(certFileNew, []byte(certNew), 0600))
	require.NoError(t, os.WriteFile(configFile, []byte(`
		encrypt = "`+gossipKeyEncoded+`"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "`+caFile+`"
		cert_file = "`+certFileNew+`"
		key_file = "`+keyFile+`"
		connect { enabled = true }
		auto_reload_config = true
	`), 0600))

	// cert should change as we did not update the associated key
	time.Sleep(1 * time.Second)
	retry.Run(t, func(r *retry.R) {
		cert := srv.tlsConfigurator.Cert()
		require.NotNil(r, cert)
		require.NotEqual(r, cert1Key, cert.Certificate)
		require.NotEqual(r, cert1Key, cert.PrivateKey)
	})
	cert2Pub := srv.tlsConfigurator.Cert().Certificate
	cert2Key := srv.tlsConfigurator.Cert().PrivateKey

	certNew2, privateKeyNew2, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, []byte(privateKeyNew2), 0600))
	// cert should not change as we did not update the associated cert
	time.Sleep(1 * time.Second)
	retry.Run(t, func(r *retry.R) {
		cert := srv.tlsConfigurator.Cert()
		require.NotNil(r, cert)
		require.Equal(r, cert2Pub, cert.Certificate)
		require.Equal(r, cert2Key, cert.PrivateKey)
	})

	require.NoError(t, os.WriteFile(certFileNew, []byte(certNew2), 0600))

	// cert should change as we did  update the associated key
	time.Sleep(1 * time.Second)
	retry.Run(t, func(r *retry.R) {
		cert := srv.tlsConfigurator.Cert()
		require.NotNil(r, cert)
		require.NotEqual(r, cert2Pub, cert.Certificate)
		require.NotEqual(r, cert2Key, cert.PrivateKey)
	})
}

func Test_coalesceTimerTwoPeriods(t *testing.T) {

	certsDir := testutil.TempDir(t, "auto-config")

	// write some test TLS certificates out to the cfg dir
	serverName := "server.dc1.consul"
	signer, _, err := tlsutil.GeneratePrivateKey()
	require.NoError(t, err)

	ca, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	require.NoError(t, err)

	cert, privateKey, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)

	certFile := filepath.Join(certsDir, "cert.pem")
	caFile := filepath.Join(certsDir, "cacert.pem")
	keyFile := filepath.Join(certsDir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, []byte(cert), 0600))
	require.NoError(t, os.WriteFile(caFile, []byte(ca), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte(privateKey), 0600))

	// generate a gossip key
	gossipKey := make([]byte, 32)
	n, err := rand.Read(gossipKey)
	require.NoError(t, err)
	require.Equal(t, 32, n)
	gossipKeyEncoded := base64.StdEncoding.EncodeToString(gossipKey)

	hclConfig := TestACLConfigWithParams(nil)

	configFile := testutil.TempDir(t, "config") + "/config.hcl"
	require.NoError(t, os.WriteFile(configFile, []byte(`
		encrypt = "`+gossipKeyEncoded+`"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "`+caFile+`"
		cert_file = "`+certFile+`"
		key_file = "`+keyFile+`"
		connect { enabled = true }
		auto_reload_config = true
	`), 0600))

	coalesceInterval := 100 * time.Millisecond
	testAgent := TestAgent{Name: "TestAgent-Server", HCL: hclConfig, configFiles: []string{configFile}, Config: &config.RuntimeConfig{
		AutoReloadConfigCoalesceInterval: coalesceInterval,
	}}
	srv := StartTestAgent(t, testAgent)
	defer srv.Shutdown()

	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	cert1Pub := srv.tlsConfigurator.Cert().Certificate
	cert1Key := srv.tlsConfigurator.Cert().PrivateKey

	certNew, privateKeyNew, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)
	certFileNew := filepath.Join(certsDir, "cert_new.pem")
	require.NoError(t, os.WriteFile(certFileNew, []byte(certNew), 0600))
	require.NoError(t, os.WriteFile(configFile, []byte(`
		encrypt = "`+gossipKeyEncoded+`"
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		verify_incoming = true
		verify_outgoing = true
		verify_server_hostname = true
		ca_file = "`+caFile+`"
		cert_file = "`+certFileNew+`"
		key_file = "`+keyFile+`"
		connect { enabled = true }
		auto_reload_config = true
	`), 0600))

	// cert should not change as we did not update the associated key
	time.Sleep(coalesceInterval * 2)
	retry.Run(t, func(r *retry.R) {
		cert := srv.tlsConfigurator.Cert()
		require.NotNil(r, cert)
		require.Equal(r, cert1Pub, cert.Certificate)
		require.Equal(r, cert1Key, cert.PrivateKey)
	})

	require.NoError(t, os.WriteFile(keyFile, []byte(privateKeyNew), 0600))

	// cert should change as we did not update the associated key
	time.Sleep(coalesceInterval * 2)
	retry.Run(t, func(r *retry.R) {
		require.NotEqual(r, cert1Pub, srv.tlsConfigurator.Cert().Certificate)
		require.NotEqual(r, cert1Key, srv.tlsConfigurator.Cert().PrivateKey)
	})

}

func TestAgent_startListeners(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	ports := freeport.GetN(t, 3)
	bd := BaseDeps{
		Deps: consul.Deps{
			Logger:       hclog.NewInterceptLogger(nil),
			Tokens:       new(token.Store),
			GRPCConnPool: &fakeGRPCConnPool{},
		},
		RuntimeConfig: &config.RuntimeConfig{
			HTTPAddrs: []net.Addr{},
		},
		Cache: cache.New(cache.Options{}),
	}

	bd, err := initEnterpriseBaseDeps(bd, nil)
	require.NoError(t, err)

	agent, err := New(bd)
	require.NoError(t, err)

	// use up an address
	used := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[2]}
	l, err := net.Listen("tcp", used.String())
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	var lns []net.Listener
	t.Cleanup(func() {
		for _, ln := range lns {
			ln.Close()
		}
	})

	// first two addresses open listeners but third address should fail
	lns, err = agent.startListeners([]net.Addr{
		&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[0]},
		&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[1]},
		&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[2]},
	})
	require.Contains(t, err.Error(), "address already in use")

	// first two ports should be freed up
	retry.Run(t, func(r *retry.R) {
		lns, err = agent.startListeners([]net.Addr{
			&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[0]},
			&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[1]},
		})
		require.NoError(r, err)
		require.Len(r, lns, 2)
	})

	// first two ports should be in use
	retry.Run(t, func(r *retry.R) {
		_, err = agent.startListeners([]net.Addr{
			&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[0]},
			&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: ports[1]},
		})
		require.Contains(r, err.Error(), "address already in use")
	})

}

func getExpectedCaPoolByFile(t *testing.T) *x509.CertPool {
	pool := x509.NewCertPool()
	data, err := os.ReadFile("../test/ca/root.cer")
	require.NoError(t, err)
	if !pool.AppendCertsFromPEM(data) {
		t.Fatal("could not add test ca ../test/ca/root.cer to pool")
	}
	return pool
}

func getExpectedCaPoolByDir(t *testing.T) *x509.CertPool {
	pool := x509.NewCertPool()
	entries, err := os.ReadDir("../test/ca_path")
	require.NoError(t, err)

	for _, entry := range entries {
		filename := path.Join("../test/ca_path", entry.Name())

		data, err := os.ReadFile(filename)
		require.NoError(t, err)

		if !pool.AppendCertsFromPEM(data) {
			t.Fatalf("could not add test ca %s to pool", filename)
		}
	}

	return pool
}

// lazyCerts has a func field which can't be compared.
var cmpCertPool = cmp.Options{
	cmpopts.IgnoreFields(x509.CertPool{}, "lazyCerts"),
	cmp.AllowUnexported(x509.CertPool{}),
}

func assertDeepEqual(t *testing.T, x, y interface{}, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(x, y, opts...); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}
