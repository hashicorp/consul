package agent

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"

	"github.com/stretchr/testify/require"
)

type authzResolver func(string) (structs.ACLIdentity, acl.Authorizer, error)
type identResolver func(string) (structs.ACLIdentity, error)

type TestACLAgent struct {
	resolveAuthzFn authzResolver
	resolveIdentFn identResolver

	*Agent
}

// NewTestACLAgent does just enough so that all the code within agent/acl.go can work
// Basically it needs a local state for some of the vet* functions, a logger and a delegate.
// The key is that we are the delegate so we can control the ResolveToken responses
func NewTestACLAgent(t *testing.T, name string, hcl string, resolveAuthz authzResolver, resolveIdent identResolver) *TestACLAgent {
	t.Helper()

	if resolveIdent == nil {
		resolveIdent = func(s string) (structs.ACLIdentity, error) {
			return nil, nil
		}
	}

	a := &TestACLAgent{resolveAuthzFn: resolveAuthz, resolveIdentFn: resolveIdent}

	dataDir := testutil.TempDir(t, "acl-agent")

	logBuffer := testutil.NewLogBuffer(t)

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:       name,
		Level:      testutil.TestLogLevel,
		Output:     logBuffer,
		TimeFormat: "04:05.000",
	})

	loader := func(source config.Source) (config.LoadResult, error) {
		dataDir := fmt.Sprintf(`data_dir = "%s"`, dataDir)
		opts := config.LoadOpts{
			HCL:           []string{TestConfigHCL(NodeID()), hcl, dataDir},
			DefaultConfig: source,
		}
		result, err := config.Load(opts)
		if result.RuntimeConfig != nil {
			result.RuntimeConfig.Telemetry.Disable = true
		}
		return result, err
	}
	bd, err := NewBaseDeps(loader, logBuffer, logger)
	require.NoError(t, err)

	bd.MetricsConfig = &lib.MetricsConfig{
		Handler: metrics.NewInmemSink(1*time.Second, time.Minute),
	}

	agent, err := New(bd)
	require.NoError(t, err)

	agent.delegate = a
	agent.State = local.NewState(LocalConfig(bd.RuntimeConfig), bd.Logger, bd.Tokens)
	agent.State.TriggerSyncChanges = func() {}
	a.Agent = agent
	return a
}

func (a *TestACLAgent) ResolveToken(secretID string) (acl.Authorizer, error) {
	if a.resolveAuthzFn == nil {
		return nil, fmt.Errorf("ResolveToken call is unexpected - no authz resolver callback set")
	}

	_, authz, err := a.resolveAuthzFn(secretID)
	return authz, err
}

func (a *TestACLAgent) ResolveTokenAndDefaultMeta(secretID string, entMeta *acl.EnterpriseMeta, authzContext *acl.AuthorizerContext) (resolver.Result, error) {
	authz, err := a.ResolveToken(secretID)
	if err != nil {
		return resolver.Result{}, err
	}

	identity, err := a.resolveIdentFn(secretID)
	if err != nil {
		return resolver.Result{}, err
	}

	// Default the EnterpriseMeta based on the Tokens meta or actual defaults
	// in the case of unknown identity
	if identity != nil {
		entMeta.Merge(identity.EnterpriseMetadata())
	} else {
		entMeta.Merge(structs.DefaultEnterpriseMetaInDefaultPartition())
	}

	// Use the meta to fill in the ACL authorization context
	entMeta.FillAuthzContext(authzContext)

	return resolver.Result{Authorizer: authz, ACLIdentity: identity}, err
}

// All of these are stubs to satisfy the interface
func (a *TestACLAgent) GetLANCoordinate() (lib.CoordinateSet, error) {
	return nil, fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) Leave() error {
	return fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) LANMembersInAgentPartition() []serf.Member {
	return nil
}
func (a *TestACLAgent) LANMembers(f consul.LANMemberFilter) ([]serf.Member, error) {
	return nil, fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) AgentLocalMember() serf.Member {
	return serf.Member{}
}
func (a *TestACLAgent) JoinLAN(addrs []string, entMeta *acl.EnterpriseMeta) (n int, err error) {
	return 0, fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) RemoveFailedNode(node string, prune bool, entMeta *acl.EnterpriseMeta) error {
	return fmt.Errorf("Unimplemented")
}
func (a *TestACLAgent) RPC(ctx context.Context, method string, args interface{}, reply interface{}) error {
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
func (a *TestACLAgent) ReloadConfig(_ consul.ReloadableConfig) error {
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

	_, err := a.delegate.ResolveTokenAndDefaultMeta("nope", nil, nil)
	require.Error(t, err)
	require.True(t, called)
}

func authzFromPolicy(policy *acl.Policy, cfg *acl.Config) (acl.Authorizer, error) {
	return acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, cfg)
}

type testTokenRules struct {
	token structs.ACLToken
	// rules to create associated policy
	rules string
}

var (
	nodeROSecret    = "7e80d017-bccc-492f-8dec-65f03aeaebf3"
	nodeRWSecret    = "e3586ee5-02a2-4bf4-9ec3-9c4be7606e8c"
	serviceROSecret = "3d2c8552-df3b-4da7-9890-36885cbf56ac"
	serviceRWSecret = "4a1017a2-f788-4be3-93f2-90566f1340bb"
	otherRWSecret   = "a38e8016-91b6-4876-b3e7-a307abbb2002"

	testACLs = map[string]testTokenRules{
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

func catalogPolicy(testACL string) (structs.ACLIdentity, acl.Authorizer, error) {
	tok, ok := testACLs[testACL]
	if !ok {
		return nil, nil, acl.ErrNotFound
	}

	policy, err := acl.NewPolicyFromSource(tok.rules, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	authz, err := authzFromPolicy(policy, nil)
	return &tok.token, authz, err
}

func catalogIdent(testACL string) (structs.ACLIdentity, error) {
	tok, ok := testACLs[testACL]
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
	a.State.AddServiceWithChecks(&structs.NodeService{
		ID:      "my-service",
		Service: "other",
	}, nil, "", false)
	err = a.vetServiceRegister(serviceRWSecret, &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetServiceUpdateWithAuthorizer(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	vetServiceUpdate := func(token string, serviceID structs.ServiceID) error {
		authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
		if err != nil {
			return err
		}

		return a.vetServiceUpdateWithAuthorizer(authz, serviceID)
	}

	// Update a service that doesn't exist.
	err := vetServiceUpdate(serviceRWSecret, structs.NewServiceID("my-service", nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown service")

	// Update with write privs.
	a.State.AddServiceWithChecks(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, nil, "", false)
	err = vetServiceUpdate(serviceRWSecret, structs.NewServiceID("my-service", nil))
	require.NoError(t, err)

	// Update without write privs.
	err = vetServiceUpdate(serviceROSecret, structs.NewServiceID("my-service", nil))
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetCheckRegisterWithAuthorizer(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	vetCheckRegister := func(token string, check *structs.HealthCheck) error {
		authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
		if err != nil {
			return err
		}
		return a.vetCheckRegisterWithAuthorizer(authz, check)
	}

	// Register a new service check with write privs.
	err := vetCheckRegister(serviceRWSecret, &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.NoError(t, err)

	// Register a new service check without write privs.
	err = vetCheckRegister(serviceROSecret, &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Register a new node check with write privs.
	err = vetCheckRegister(nodeRWSecret, &structs.HealthCheck{
		CheckID: types.CheckID("my-check"),
	})
	require.NoError(t, err)

	// Register a new node check without write privs.
	err = vetCheckRegister(nodeROSecret, &structs.HealthCheck{
		CheckID: types.CheckID("my-check"),
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Try to register over a service check without write privs to the
	// existing service.
	a.State.AddServiceWithChecks(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, nil, "", false)
	a.State.AddCheck(&structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "other",
	}, "", false)
	err = vetCheckRegister(serviceRWSecret, &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Try to register over a node check without write privs to the node.
	a.State.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "", false)
	err = vetCheckRegister(serviceRWSecret, &structs.HealthCheck{
		CheckID:     types.CheckID("my-node-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestACL_vetCheckUpdateWithAuthorizer(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	vetCheckUpdate := func(token string, checkID structs.CheckID) error {
		authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
		if err != nil {
			return err
		}

		return a.vetCheckUpdateWithAuthorizer(authz, checkID)
	}

	// Update a check that doesn't exist.
	err := vetCheckUpdate(nodeRWSecret, structs.NewCheckID("my-check", nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown check")

	// Update service check with write privs.
	a.State.AddServiceWithChecks(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, nil, "", false)
	a.State.AddCheck(&structs.HealthCheck{
		CheckID:     types.CheckID("my-service-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	}, "", false)
	err = vetCheckUpdate(serviceRWSecret, structs.NewCheckID("my-service-check", nil))
	require.NoError(t, err)

	// Update service check without write privs.
	err = vetCheckUpdate(serviceROSecret, structs.NewCheckID("my-service-check", nil))
	require.Error(t, err)
	require.True(t, acl.IsErrPermissionDenied(err), "not permission denied: %s", err.Error())

	// Update node check with write privs.
	a.State.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "", false)
	err = vetCheckUpdate(nodeRWSecret, structs.NewCheckID("my-node-check", nil))
	require.NoError(t, err)

	// Update without write privs.
	err = vetCheckUpdate(nodeROSecret, structs.NewCheckID("my-node-check", nil))
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

func TestACL_filterServicesWithAuthorizer(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	filterServices := func(token string, services map[string]*api.AgentService) error {
		authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
		if err != nil {
			return err
		}

		return a.filterServicesWithAuthorizer(authz, services)
	}

	services := make(map[string]*api.AgentService)
	require.NoError(t, filterServices(nodeROSecret, services))

	services[structs.NewServiceID("my-service", nil).String()] = &api.AgentService{ID: "my-service", Service: "service"}
	services[structs.NewServiceID("my-other", nil).String()] = &api.AgentService{ID: "my-other", Service: "other"}
	require.NoError(t, filterServices(serviceROSecret, services))

	require.Contains(t, services, structs.NewServiceID("my-service", nil).String())
	require.NotContains(t, services, structs.NewServiceID("my-other", nil).String())
}

func TestACL_filterChecksWithAuthorizer(t *testing.T) {
	t.Parallel()
	a := NewTestACLAgent(t, t.Name(), TestACLConfig(), catalogPolicy, catalogIdent)

	filterChecks := func(token string, checks map[types.CheckID]*structs.HealthCheck) error {
		authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
		if err != nil {
			return err
		}

		return a.filterChecksWithAuthorizer(authz, checks)
	}

	checks := make(map[types.CheckID]*structs.HealthCheck)
	require.NoError(t, filterChecks(nodeROSecret, checks))

	checks["my-node"] = &structs.HealthCheck{}
	checks["my-service"] = &structs.HealthCheck{ServiceName: "service"}
	checks["my-other"] = &structs.HealthCheck{ServiceName: "other"}
	require.NoError(t, filterChecks(serviceROSecret, checks))
	_, ok := checks["my-node"]
	require.False(t, ok)
	_, ok = checks["my-service"]
	require.True(t, ok)
	_, ok = checks["my-other"]
	require.False(t, ok)

	checks["my-node"] = &structs.HealthCheck{}
	checks["my-service"] = &structs.HealthCheck{ServiceName: "service"}
	checks["my-other"] = &structs.HealthCheck{ServiceName: "other"}
	require.NoError(t, filterChecks(nodeROSecret, checks))
	_, ok = checks["my-node"]
	require.True(t, ok)
	_, ok = checks["my-service"]
	require.False(t, ok)
	_, ok = checks["my-other"]
	require.False(t, ok)
}
