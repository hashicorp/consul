package agent

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"

	"github.com/stretchr/testify/require"
)

type authzResolver func(string) (structs.ACLIdentity, acl.Authorizer, error)
type identResolver func(string) (structs.ACLIdentity, error)

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

	// DataDir is the data directory which is used when Config.DataDir
	// is not set. It is created automatically and removed when
	// Shutdown() is called.
	DataDir string

	resolveAuthzFn authzResolver
	resolveIdentFn identResolver

	*Agent
}

// NewTestACLAgent does just enough so that all the code within agent/acl.go can work
// Basically it needs a local state for some of the vet* functions, a logger and a delegate.
// The key is that we are the delegate so we can control the ResolveToken responses
func NewTestACLAgent(t *testing.T, name string, hcl string, resolveAuthz authzResolver, resolveIdent identResolver) *TestACLAgent {
	a := &TestACLAgent{Name: name, HCL: hcl, resolveAuthzFn: resolveAuthz, resolveIdentFn: resolveIdent}
	dataDir := `data_dir = "acl-agent"`

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:   a.Name,
		Level:  hclog.Debug,
		Output: testutil.NewLogBuffer(t),
	})

	opts := []AgentOption{
		WithLogger(logger),
		WithBuilderOpts(config.BuilderOpts{
			HCL: []string{
				TestConfigHCL(NodeID()),
				a.HCL,
				dataDir,
			},
		}),
	}

	agent, err := New(opts...)
	require.NoError(t, err)
	a.Config = agent.GetConfig()
	a.Agent = agent

	agent.logger = logger
	agent.MemSink = metrics.NewInmemSink(1*time.Second, time.Minute)

	a.Agent.delegate = a
	a.Agent.State = local.NewState(LocalConfig(a.Config), a.Agent.logger, a.Agent.tokens)
	a.Agent.State.TriggerSyncChanges = func() {}
	return a
}

func (a *TestACLAgent) UseLegacyACLs() bool {
	return false
}

func (a *TestACLAgent) ResolveToken(secretID string) (acl.Authorizer, error) {
	if a.resolveAuthzFn == nil {
		return nil, fmt.Errorf("ResolveToken call is unexpected - no authz resolver callback set")
	}

	_, authz, err := a.resolveAuthzFn(secretID)
	return authz, err
}

func (a *TestACLAgent) ResolveTokenToIdentityAndAuthorizer(secretID string) (structs.ACLIdentity, acl.Authorizer, error) {
	if a.resolveAuthzFn == nil {
		return nil, nil, fmt.Errorf("ResolveTokenToIdentityAndAuthorizer call is unexpected - no authz resolver callback set")
	}

	return a.resolveAuthzFn(secretID)
}

func (a *TestACLAgent) ResolveTokenToIdentity(secretID string) (structs.ACLIdentity, error) {
	if a.resolveIdentFn == nil {
		return nil, fmt.Errorf("ResolveTokenToIdentity call is unexpected - no ident resolver callback set")
	}

	return a.resolveIdentFn(secretID)
}

func (a *TestACLAgent) ResolveTokenAndDefaultMeta(secretID string, entMeta *structs.EnterpriseMeta, authzContext *acl.AuthorizerContext) (acl.Authorizer, error) {
	identity, authz, err := a.ResolveTokenToIdentityAndAuthorizer(secretID)
	if err != nil {
		return nil, err
	}

	// Default the EnterpriseMeta based on the Tokens meta or actual defaults
	// in the case of unknown identity
	if identity != nil {
		entMeta.Merge(identity.EnterpriseMetadata())
	} else {
		entMeta.Merge(structs.DefaultEnterpriseMeta())
	}

	// Use the meta to fill in the ACL authorization context
	entMeta.FillAuthzContext(authzContext)

	return authz, err
}

// All of these are stubs to satisfy the interface
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
func (a *TestACLAgent) RemoveFailedNode(node string, prune bool) error {
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

func TestACL_Version8EnabledByDefault(t *testing.T) {
	t.Parallel()

	called := false
	resolveFn := func(string) (structs.ACLIdentity, acl.Authorizer, error) {
		called = true
		return nil, nil, acl.ErrNotFound
	}
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), resolveFn, nil)

	_, err := a.resolveToken("nope")
	require.Error(t, err)
	require.True(t, called)
}

func TestACL_AgentMasterToken(t *testing.T) {
	t.Parallel()

	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), nil, nil)
	a.loadTokens(a.config)
	authz, err := a.resolveToken("towel")
	require.NotNil(t, authz)
	require.Nil(t, err)

	require.Equal(t, acl.Allow, authz.AgentRead(a.config.NodeName, nil))
	require.Equal(t, acl.Allow, authz.AgentWrite(a.config.NodeName, nil))
	require.Equal(t, acl.Allow, authz.NodeRead("foobarbaz", nil))
	require.Equal(t, acl.Deny, authz.NodeWrite("foobarbaz", nil))
}

func TestACL_RootAuthorizersDenied(t *testing.T) {
	t.Parallel()

	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), nil, nil)
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

func authzFromPolicy(policy *acl.Policy, cfg *acl.Config) (acl.Authorizer, error) {
	return acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, cfg)
}

type testToken struct {
	token structs.ACLToken
	// yes the rules can exist on the token itself but that is legacy behavior
	// that I would prefer these tests not rely on
	rules string
}

var (
	nodeROSecret    = "7e80d017-bccc-492f-8dec-65f03aeaebf3"
	nodeRWSecret    = "e3586ee5-02a2-4bf4-9ec3-9c4be7606e8c"
	serviceROSecret = "3d2c8552-df3b-4da7-9890-36885cbf56ac"
	serviceRWSecret = "4a1017a2-f788-4be3-93f2-90566f1340bb"
	otherRWSecret   = "a38e8016-91b6-4876-b3e7-a307abbb2002"

	testTokens = map[string]testToken{
		nodeROSecret: {
			token: structs.ACLToken{
				AccessorID: "9df2d1a4-2d07-414e-8ead-6053f56ed2eb",
				SecretID:   nodeROSecret,
			},
			rules: `node_prefix "Node" { policy = "read" }`,
		},
		nodeRWSecret: {
			token: structs.ACLToken{
				AccessorID: "efb6b7d5-d343-47c1-b4cb-aa6b94d2f490",
				SecretID:   nodeRWSecret,
			},
			rules: `node_prefix "Node" { policy = "write" }`,
		},
		serviceROSecret: {
			token: structs.ACLToken{
				AccessorID: "0da53edb-36e5-4603-9c31-79965bad45f5",
				SecretID:   serviceROSecret,
			},
			rules: `service_prefix "service" { policy = "read" }`,
		},
		serviceRWSecret: {
			token: structs.ACLToken{
				AccessorID: "52504258-137a-41e6-9326-01f40e80872e",
				SecretID:   serviceRWSecret,
			},
			rules: `service_prefix "service" { policy = "write" }`,
		},
		otherRWSecret: {
			token: structs.ACLToken{
				AccessorID: "5e032c5b-c39e-4552-b5ad-8a9365b099c4",
				SecretID:   otherRWSecret,
			},
			rules: `service_prefix "other" { policy = "write" }`,
		},
	}
)

func catalogPolicy(token string) (structs.ACLIdentity, acl.Authorizer, error) {
	tok, ok := testTokens[token]
	if !ok {
		return nil, nil, acl.ErrNotFound
	}

	policy, err := acl.NewPolicyFromSource("", 0, tok.rules, acl.SyntaxCurrent, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	authz, err := authzFromPolicy(policy, nil)
	return &tok.token, authz, err
}

func catalogIdent(token string) (structs.ACLIdentity, error) {
	tok, ok := testTokens[token]
	if !ok {
		return nil, acl.ErrNotFound
	}

	return &tok.token, nil
}

func TestACL_vetServiceRegister(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	// Register a new service, with permission.
	err := a.vetServiceRegister(serviceRWSecret, &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	require.NoError(t, err)

	// Register a new service without write privs.
	err = a.vetServiceRegister(serviceROSecret, &structs.NodeService{
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
	err = a.vetServiceRegister(serviceRWSecret, &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetServiceUpdate(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	// Update a service that doesn't exist.
	err := a.vetServiceUpdate(serviceRWSecret, structs.NewServiceID("my-service", nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown service")

	// Update with write privs.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	err = a.vetServiceUpdate(serviceRWSecret, structs.NewServiceID("my-service", nil))
	require.NoError(t, err)

	// Update without write privs.
	err = a.vetServiceUpdate(serviceROSecret, structs.NewServiceID("my-service", nil))
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetCheckRegister(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	// Register a new service check with write privs.
	err := a.vetCheckRegister(serviceRWSecret, &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.NoError(t, err)

	// Register a new service check without write privs.
	err = a.vetCheckRegister(serviceROSecret, &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Register a new node check with write privs.
	err = a.vetCheckRegister(nodeRWSecret, &structs.HealthCheck{
		CheckID: types.CheckID("my-check"),
	})
	require.NoError(t, err)

	// Register a new node check without write privs.
	err = a.vetCheckRegister(nodeROSecret, &structs.HealthCheck{
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
	err = a.vetCheckRegister(serviceRWSecret, &structs.HealthCheck{
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
	err = a.vetCheckRegister(serviceRWSecret, &structs.HealthCheck{
		CheckID:     types.CheckID("my-node-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetCheckUpdate(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	// Update a check that doesn't exist.
	err := a.vetCheckUpdate(nodeRWSecret, structs.NewCheckID("my-check", nil))
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
	err = a.vetCheckUpdate(serviceRWSecret, structs.NewCheckID("my-service-check", nil))
	require.NoError(t, err)

	// Update service check without write privs.
	err = a.vetCheckUpdate(serviceROSecret, structs.NewCheckID("my-service-check", nil))
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err), "not permission denied: %s", err.Error())

	// Update node check with write privs.
	a.State.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "")
	err = a.vetCheckUpdate(nodeRWSecret, structs.NewCheckID("my-node-check", nil))
	require.NoError(t, err)

	// Update without write privs.
	err = a.vetCheckUpdate(nodeROSecret, structs.NewCheckID("my-node-check", nil))
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_filterMembers(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	var members []serf.Member
	require.NoError(t, a.filterMembers(nodeROSecret, &members))
	require.Len(t, members, 0)

	members = []serf.Member{
		{Name: "Node 1"},
		{Name: "Nope"},
		{Name: "Node 2"},
	}
	require.NoError(t, a.filterMembers(nodeROSecret, &members))
	require.Len(t, members, 2)
	require.Equal(t, members[0].Name, "Node 1")
	require.Equal(t, members[1].Name, "Node 2")
}

func TestACL_filterServices(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	services := make(map[structs.ServiceID]*structs.NodeService)
	require.NoError(t, a.filterServices(nodeROSecret, &services))

	services[structs.NewServiceID("my-service", nil)] = &structs.NodeService{ID: "my-service", Service: "service"}
	services[structs.NewServiceID("my-other", nil)] = &structs.NodeService{ID: "my-other", Service: "other"}
	require.NoError(t, a.filterServices(serviceROSecret, &services))
	require.Contains(t, services, structs.NewServiceID("my-service", nil))
	require.NotContains(t, services, structs.NewServiceID("my-other", nil))
}

func TestACL_filterChecks(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	checks := make(map[structs.CheckID]*structs.HealthCheck)
	require.NoError(t, a.filterChecks(nodeROSecret, &checks))

	checks[structs.NewCheckID("my-node", nil)] = &structs.HealthCheck{}
	checks[structs.NewCheckID("my-service", nil)] = &structs.HealthCheck{ServiceName: "service"}
	checks[structs.NewCheckID("my-other", nil)] = &structs.HealthCheck{ServiceName: "other"}
	require.NoError(t, a.filterChecks(serviceROSecret, &checks))
	_, ok := checks[structs.NewCheckID("my-node", nil)]
	require.False(t, ok)
	_, ok = checks[structs.NewCheckID("my-service", nil)]
	require.True(t, ok)
	_, ok = checks[structs.NewCheckID("my-other", nil)]
	require.False(t, ok)

	checks[structs.NewCheckID("my-node", nil)] = &structs.HealthCheck{}
	checks[structs.NewCheckID("my-service", nil)] = &structs.HealthCheck{ServiceName: "service"}
	checks[structs.NewCheckID("my-other", nil)] = &structs.HealthCheck{ServiceName: "other"}
	require.NoError(t, a.filterChecks(nodeROSecret, &checks))
	_, ok = checks[structs.NewCheckID("my-node", nil)]
	require.True(t, ok)
	_, ok = checks[structs.NewCheckID("my-service", nil)]
	require.False(t, ok)
	_, ok = checks[structs.NewCheckID("my-other", nil)]
	require.False(t, ok)
}

func TestACL_ResolveIdentity(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), nil, catalogIdent)

	// this test is meant to ensure we are calling the correct function
	// which is ResolveTokenToIdentity on the Agent delegate. Our
	// nil authz resolver will cause it to emit an error if used
	ident, err := a.resolveIdentityFromToken(nodeROSecret)
	require.NoError(t, err)
	require.NotNil(t, ident)

	// just double checkingto ensure if we had used the wrong function
	// that an error would be produced
	_, err = a.resolveToken(nodeROSecret)
	require.Error(t, err)

}
