package agent

import (
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/serf/serf"

	"github.com/stretchr/testify/require"
)

type TestACLAgent struct {
	// Name is an optional name of the agent.
	Name string

	HCL string

	// Config is the agent configuration. If Config is nil then
	// TestConfig() is used. If Config.DataDir is set then it is
	// the callers responsibility to clean up the data directory.
	// Otherwise, a temporary data directory is created and removed
	// when Shutdown() is called.
	Config *config.RuntimeConfig

	// LogOutput is the sink for the logs. If nil, logs are written
	// to os.Stderr.
	LogOutput io.Writer

	// LogWriter is used for streaming logs.
	LogWriter *logger.LogWriter

	// DataDir is the data directory which is used when Config.DataDir
	// is not set. It is created automatically and removed when
	// Shutdown() is called.
	DataDir string

	resolveTokenFn func(string) (acl.Authorizer, error)

	*Agent
}

// NewTestACLAGent does just enough so that all the code within agent/acl.go can work
// Basically it needs a local state for some of the vet* functions, a logger and a delegate.
// The key is that we are the delegate so we can control the ResolveToken responses
func NewTestACLAgent(name string, hcl string, resolveFn func(string) (acl.Authorizer, error)) *TestACLAgent {
	a := &TestACLAgent{Name: name, HCL: hcl, resolveTokenFn: resolveFn}
	hclDataDir := `data_dir = "acl-agent"`

	a.Config = TestConfig(
		config.Source{Name: a.Name, Format: "hcl", Data: a.HCL},
		config.Source{Name: a.Name + ".data_dir", Format: "hcl", Data: hclDataDir},
	)

	agent, err := New(a.Config, nil)
	if err != nil {
		panic(fmt.Sprintf("Error creating agent: %v", err))
	}
	a.Agent = agent

	logOutput := a.LogOutput
	if logOutput == nil {
		logOutput = os.Stderr
	}
	agent.LogOutput = logOutput
	agent.LogWriter = a.LogWriter
	agent.logger = log.New(logOutput, a.Name+" - ", log.LstdFlags|log.Lmicroseconds)
	agent.MemSink = metrics.NewInmemSink(1*time.Second, time.Minute)

	a.Agent.delegate = a
	a.Agent.State = local.NewState(LocalConfig(a.Config), a.Agent.logger, a.Agent.tokens)
	a.Agent.State.TriggerSyncChanges = func() {}
	return a
}

func (a *TestACLAgent) ACLsEnabled() bool {
	// the TestACLAgent always has ACLs enabled
	return true
}

func (a *TestACLAgent) UseLegacyACLs() bool {
	return false
}

func (a *TestACLAgent) ResolveToken(secretID string) (acl.Authorizer, error) {
	if a.resolveTokenFn == nil {
		panic("This agent is useless without providing a token resolution function")
	}

	return a.resolveTokenFn(secretID)
}

// All of these are stubs to satisfy the interface
func (a *TestACLAgent) Encrypted() bool {
	return false
}
func (a *TestACLAgent) GetLANCoordinate() (lib.CoordinateSet, error) {
	return nil, fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) Leave() error {
	return fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) LANMembers() []serf.Member {
	return nil
}
func (a *TestACLAgent) LANMembersAllSegments() ([]serf.Member, error) {
	return nil, fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) LANSegmentMembers(segment string) ([]serf.Member, error) {
	return nil, fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) LocalMember() serf.Member {
	return serf.Member{}
}
func (a *TestACLAgent) JoinLAN(addrs []string) (n int, err error) {
	return 0, fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) RemoveFailedNode(node string) error {
	return fmt.Errorf("Unimplemented")
}

func (a *TestACLAgent) RPC(method string, args interface{}, reply interface{}) error {
	return fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer, replyFn structs.SnapshotReplyFn) error {
	return fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) Shutdown() error {
	return fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) Stats() map[string]map[string]string {
	return nil
}
func (a *TestACLAgent) ReloadConfig(config *consul.Config) error {
	return fmt.Errorf("Unimplemented")
}

func TestACL_Version8(t *testing.T) {
	t.Parallel()

	t.Run("version 8 disabled", func(t *testing.T) {
		resolveFn := func(string) (acl.Authorizer, error) {
			require.Fail(t, "should not have called delegate.ResolveToken")
			return nil, fmt.Errorf("should not have called delegate.ResolveToken")
		}

		a := NewTestACLAgent(t.Name(), TestACLConfig()+`
 		acl_enforce_version_8 = false
 	`, resolveFn)

		token, err := a.resolveToken("nope")
		require.Nil(t, token)
		require.Nil(t, err)
	})

	t.Run("version 8 enabled", func(t *testing.T) {
		called := false
		resolveFn := func(string) (acl.Authorizer, error) {
			called = true
			return nil, acl.ErrNotFound
		}
		a := NewTestACLAgent(t.Name(), TestACLConfig()+`
 		acl_enforce_version_8 = true
 	`, resolveFn)

		_, err := a.resolveToken("nope")
		require.Error(t, err)
		require.True(t, called)
	})
}

func TestACL_AgentMasterToken(t *testing.T) {
	t.Parallel()

	resolveFn := func(string) (acl.Authorizer, error) {
		require.Fail(t, "should not have called delegate.ResolveToken")
		return nil, fmt.Errorf("should not have called delegate.ResolveToken")
	}

	a := NewTestACLAgent(t.Name(), TestACLConfig(), resolveFn)
	a.loadTokens(a.config)
	authz, err := a.resolveToken("towel")
	require.NotNil(t, authz)
	require.Nil(t, err)

	require.True(t, authz.AgentRead(a.config.NodeName))
	require.True(t, authz.AgentWrite(a.config.NodeName))
	require.True(t, authz.NodeRead("foobarbaz"))
	require.False(t, authz.NodeWrite("foobarbaz", nil))
}

func TestACL_RootAuthorizersDenied(t *testing.T) {
	t.Parallel()

	resolveFn := func(string) (acl.Authorizer, error) {
		require.Fail(t, "should not have called delegate.ResolveToken")
		return nil, fmt.Errorf("should not have called delegate.ResolveToken")
	}

	a := NewTestACLAgent(t.Name(), TestACLConfig(), resolveFn)
	authz, err := a.resolveToken("deny")
	require.Nil(t, authz)
	require.Error(t, err)
	require.True(t, acl.IsErrRootDenied(err))
	authz, err = a.resolveToken("allow")
	require.Nil(t, authz)
	require.Error(t, err)
	require.True(t, acl.IsErrRootDenied(err))
	authz, err = a.resolveToken("manage")
	require.Nil(t, authz)
	require.Error(t, err)
	require.True(t, acl.IsErrRootDenied(err))
}

func authzFromPolicy(policy *acl.Policy) (acl.Authorizer, error) {
	return acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
}

// catalogPolicy supplies some standard policies to help with testing the
// catalog-related vet and filter functions.
func catalogPolicy(token string) (acl.Authorizer, error) {
	switch token {

	case "node-ro":
		return authzFromPolicy(&acl.Policy{
			NodePrefixes: []*acl.NodePolicy{
				&acl.NodePolicy{Name: "Node", Policy: "read"},
			},
		})
	case "node-rw":
		return authzFromPolicy(&acl.Policy{
			NodePrefixes: []*acl.NodePolicy{
				&acl.NodePolicy{Name: "Node", Policy: "write"},
			},
		})
	case "service-ro":
		return authzFromPolicy(&acl.Policy{
			ServicePrefixes: []*acl.ServicePolicy{
				&acl.ServicePolicy{Name: "service", Policy: "read"},
			},
		})
	case "service-rw":
		return authzFromPolicy(&acl.Policy{
			ServicePrefixes: []*acl.ServicePolicy{
				&acl.ServicePolicy{Name: "service", Policy: "write"},
			},
		})
	case "other-rw":
		return authzFromPolicy(&acl.Policy{
			ServicePrefixes: []*acl.ServicePolicy{
				&acl.ServicePolicy{Name: "other", Policy: "write"},
			},
		})
	default:
		return nil, fmt.Errorf("unknown token %q", token)
	}
}

func TestACL_vetServiceRegister(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t.Name(), TestACLConfig(), catalogPolicy)

	// Register a new service, with permission.
	err := a.vetServiceRegister("service-rw", &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	require.NoError(t, err)

	// Register a new service without write privs.
	err = a.vetServiceRegister("service-ro", &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	require.True(t, acl.IsErrPermissionDenied(err))

	// Try to register over a service without write privs to the existing
	// service.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "other",
	}, "")
	err = a.vetServiceRegister("service-rw", &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetServiceUpdate(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t.Name(), TestACLConfig(), catalogPolicy)

	// Update a service that doesn't exist.
	err := a.vetServiceUpdate("service-rw", "my-service")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown service")

	// Update with write privs.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	err = a.vetServiceUpdate("service-rw", "my-service")
	require.NoError(t, err)

	// Update without write privs.
	err = a.vetServiceUpdate("service-ro", "my-service")
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetCheckRegister(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t.Name(), TestACLConfig(), catalogPolicy)

	// Register a new service check with write privs.
	err := a.vetCheckRegister("service-rw", &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.NoError(t, err)

	// Register a new service check without write privs.
	err = a.vetCheckRegister("service-ro", &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Register a new node check with write privs.
	err = a.vetCheckRegister("node-rw", &structs.HealthCheck{
		CheckID: types.CheckID("my-check"),
	})
	require.NoError(t, err)

	// Register a new node check without write privs.
	err = a.vetCheckRegister("node-ro", &structs.HealthCheck{
		CheckID: types.CheckID("my-check"),
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Try to register over a service check without write privs to the
	// existing service.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	a.State.AddCheck(&structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "other",
	}, "")
	err = a.vetCheckRegister("service-rw", &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Try to register over a node check without write privs to the node.
	a.State.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "")
	err = a.vetCheckRegister("service-rw", &structs.HealthCheck{
		CheckID:     types.CheckID("my-node-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetCheckUpdate(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t.Name(), TestACLConfig(), catalogPolicy)

	// Update a check that doesn't exist.
	err := a.vetCheckUpdate("node-rw", "my-check")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown check")

	// Update service check with write privs.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	a.State.AddCheck(&structs.HealthCheck{
		CheckID:     types.CheckID("my-service-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	}, "")
	err = a.vetCheckUpdate("service-rw", "my-service-check")
	require.NoError(t, err)

	// Update service check without write privs.
	err = a.vetCheckUpdate("service-ro", "my-service-check")
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Update node check with write privs.
	a.State.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "")
	err = a.vetCheckUpdate("node-rw", "my-node-check")
	require.NoError(t, err)

	// Update without write privs.
	err = a.vetCheckUpdate("node-ro", "my-node-check")
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_filterMembers(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t.Name(), TestACLConfig(), catalogPolicy)

	var members []serf.Member
	require.NoError(t, a.filterMembers("node-ro", &members))
	require.Len(t, members, 0)

	members = []serf.Member{
		serf.Member{Name: "Node 1"},
		serf.Member{Name: "Nope"},
		serf.Member{Name: "Node 2"},
	}
	require.NoError(t, a.filterMembers("node-ro", &members))
	require.Len(t, members, 2)
	require.Equal(t, members[0].Name, "Node 1")
	require.Equal(t, members[1].Name, "Node 2")
}

func TestACL_filterServices(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t.Name(), TestACLConfig(), catalogPolicy)

	services := make(map[string]*structs.NodeService)
	require.NoError(t, a.filterServices("node-ro", &services))

	services["my-service"] = &structs.NodeService{ID: "my-service", Service: "service"}
	services["my-other"] = &structs.NodeService{ID: "my-other", Service: "other"}
	require.NoError(t, a.filterServices("service-ro", &services))
	require.Contains(t, services, "my-service")
	require.NotContains(t, services, "my-other")
}

func TestACL_filterChecks(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t.Name(), TestACLConfig(), catalogPolicy)

	checks := make(map[types.CheckID]*structs.HealthCheck)
	require.NoError(t, a.filterChecks("node-ro", &checks))

	checks["my-node"] = &structs.HealthCheck{}
	checks["my-service"] = &structs.HealthCheck{ServiceName: "service"}
	checks["my-other"] = &structs.HealthCheck{ServiceName: "other"}
	require.NoError(t, a.filterChecks("service-ro", &checks))
	fmt.Printf("filtered: %#v", checks)
	_, ok := checks["my-node"]
	require.False(t, ok)
	_, ok = checks["my-service"]
	require.True(t, ok)
	_, ok = checks["my-other"]
	require.False(t, ok)

	checks["my-node"] = &structs.HealthCheck{}
	checks["my-service"] = &structs.HealthCheck{ServiceName: "service"}
	checks["my-other"] = &structs.HealthCheck{ServiceName: "other"}
	require.NoError(t, a.filterChecks("node-ro", &checks))
	_, ok = checks["my-node"]
	require.True(t, ok)
	_, ok = checks["my-service"]
	require.False(t, ok)
	_, ok = checks["my-other"]
	require.False(t, ok)
}
