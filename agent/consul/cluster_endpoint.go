package consul

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/agentpb"
	"github.com/hashicorp/consul/agent/agentpb/config"
	"github.com/hashicorp/consul/agent/consul/authmethod/ssoauth"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/template"
	"github.com/hashicorp/consul/tlsutil"
	bexpr "github.com/hashicorp/go-bexpr"
)

type AutoConfigOptions struct {
	NodeName    string
	SegmentName string
}

type AutoConfigAuthorizer interface {
	// Authorizes the request and returns a struct containing the various
	// options for how to generate the configuration.
	Authorize(*agentpb.AutoConfigRequest) (AutoConfigOptions, error)
}

type disabledAuthorizer struct{}

func (_ *disabledAuthorizer) Authorize(_ *agentpb.AutoConfigRequest) (AutoConfigOptions, error) {
	return AutoConfigOptions{}, fmt.Errorf("Auto Config is disabled")
}

type jwtAuthorizer struct {
	validator       *ssoauth.Validator
	allowReuse      bool
	claimAssertions []string
}

func (a *jwtAuthorizer) Authorize(req *agentpb.AutoConfigRequest) (AutoConfigOptions, error) {
	// perform basic JWT Authorization
	identity, err := a.validator.ValidateLogin(context.Background(), req.JWT)
	if err != nil {
		// TODO (autoconf) maybe we should add a more generic permission denied error not tied to the ACL package?
		return AutoConfigOptions{}, acl.PermissionDenied("Failed JWT authorization: %v", err)
	}

	varMap := map[string]string{
		"node":    req.Node,
		"segment": req.Segment,
	}

	// TODO (autoconf) check for JWT reuse if configured to do so.
	for _, raw := range a.claimAssertions {
		// validate and fill any HIL
		filled, err := template.InterpolateHIL(raw, varMap, true)
		if err != nil {
			return AutoConfigOptions{}, fmt.Errorf("Failed to render claim assertion template %q: %w", raw, err)
		}

		evaluator, err := bexpr.CreateEvaluatorForType(filled, nil, identity.SelectableFields)
		if err != nil {
			return AutoConfigOptions{}, fmt.Errorf("Failed to create evaluator for claim assertion %q: %w", filled, err)
		}

		ok, err := evaluator.Evaluate(identity.SelectableFields)
		if err != nil {
			return AutoConfigOptions{}, fmt.Errorf("Failed to execute claim assertion %q: %w", filled, err)
		}

		if !ok {
			return AutoConfigOptions{}, acl.PermissionDenied("Failed JWT claim assertion")
		}
	}

	return AutoConfigOptions{
		NodeName:    req.Node,
		SegmentName: req.Segment,
	}, nil
}

// Cluster endpoint is used for cluster configuration operations
type Cluster struct {
	srv *Server

	authorizer AutoConfigAuthorizer
}

// updateTLSCertificatesInConfig will ensure that the TLS settings regarding how an agent is
// made aware of its certificates are populated. This will only work if connect is enabled and
// in some cases only if auto_encrypt is enabled on the servers. This endpoint has the option
// to configure auto_encrypt or potentially in the future to generate the certificates inline.
func (c *Cluster) updateTLSCertificatesInConfig(opts AutoConfigOptions, conf *config.Config) error {
	if c.srv.config.AutoEncryptAllowTLS {
		conf.AutoEncrypt = &config.AutoEncrypt{TLS: true}
	} else {
		conf.AutoEncrypt = &config.AutoEncrypt{TLS: false}
	}

	return nil
}

// updateACLtokensInConfig will configure all of the agents ACL settings and will populate
// the configuration with an agent token usable for all default agent operations.
func (c *Cluster) updateACLsInConfig(opts AutoConfigOptions, conf *config.Config) error {
	acl := &config.ACL{
		Enabled:             c.srv.config.ACLsEnabled,
		PolicyTTL:           c.srv.config.ACLPolicyTTL.String(),
		RoleTTL:             c.srv.config.ACLRoleTTL.String(),
		TokenTTL:            c.srv.config.ACLTokenTTL.String(),
		DisabledTTL:         c.srv.config.ACLDisabledTTL.String(),
		DownPolicy:          c.srv.config.ACLDownPolicy,
		DefaultPolicy:       c.srv.config.ACLDefaultPolicy,
		EnableKeyListPolicy: c.srv.config.ACLEnableKeyListPolicy,
	}

	// when ACLs are enabled we want to create a local token with a node identity
	if c.srv.config.ACLsEnabled {
		// we have to require local tokens or else it would require having these servers use a token with acl:write to make a
		// token create RPC to the servers in the primary DC.
		if !c.srv.LocalTokensEnabled() {
			return fmt.Errorf("Agent Auto Configuration requires local token usage to be enabled in this datacenter: %s", c.srv.config.Datacenter)
		}

		// generate the accessor id
		accessor, err := lib.GenerateUUID(c.srv.checkTokenUUID)
		if err != nil {
			return err
		}
		// generate the secret id
		secret, err := lib.GenerateUUID(c.srv.checkTokenUUID)
		if err != nil {
			return err
		}

		// set up the token
		token := structs.ACLToken{
			AccessorID:  accessor,
			SecretID:    secret,
			Description: fmt.Sprintf("Auto Config Token for Node %q", opts.NodeName),
			CreateTime:  time.Now(),
			Local:       true,
			NodeIdentities: []*structs.ACLNodeIdentity{
				{
					NodeName:   opts.NodeName,
					Datacenter: c.srv.config.Datacenter,
				},
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
		}

		req := structs.ACLTokenBatchSetRequest{
			Tokens: structs.ACLTokens{&token},
			CAS:    false,
		}

		// perform the request to mint the new token
		if _, err := c.srv.raftApplyMsgpack(structs.ACLTokenSetRequestType, &req); err != nil {
			return err
		}

		acl.Tokens = &config.ACLTokens{Agent: secret}
	}

	conf.ACL = acl
	return nil
}

// updateJoinAddressesInConfig determines the correct gossip endpoints that clients should
// be connecting to for joining the cluster based on the segment given in the opts parameter.
func (c *Cluster) updateJoinAddressesInConfig(opts AutoConfigOptions, conf *config.Config) error {
	members, err := c.srv.LANSegmentMembers(opts.SegmentName)
	if err != nil {
		return err
	}

	var joinAddrs []string
	for _, m := range members {
		if ok, _ := metadata.IsConsulServer(m); ok {
			serfAddr := net.TCPAddr{IP: m.Addr, Port: int(m.Port)}
			joinAddrs = append(joinAddrs, serfAddr.String())
		}
	}

	if conf.Gossip == nil {
		conf.Gossip = &config.Gossip{}
	}

	conf.Gossip.RetryJoinLAN = joinAddrs
	return nil
}

// updateGossipEncryptionInConfig will populate the gossip encryption configuration settings
func (c *Cluster) updateGossipEncryptionInConfig(_ AutoConfigOptions, conf *config.Config) error {
	// Add gossip encryption settings if there is any key loaded
	memberlistConfig := c.srv.config.SerfLANConfig.MemberlistConfig
	if lanKeyring := memberlistConfig.Keyring; lanKeyring != nil {
		if conf.Gossip == nil {
			conf.Gossip = &config.Gossip{}
		}
		if conf.Gossip.Encryption == nil {
			conf.Gossip.Encryption = &config.GossipEncryption{}
		}

		pk := lanKeyring.GetPrimaryKey()
		if len(pk) > 0 {
			conf.Gossip.Encryption.Key = base64.StdEncoding.EncodeToString(pk)
		}

		conf.Gossip.Encryption.VerifyIncoming = memberlistConfig.GossipVerifyIncoming
		conf.Gossip.Encryption.VerifyOutgoing = memberlistConfig.GossipVerifyOutgoing
	}

	return nil
}

// updateTLSSettingsInConfig will populate the TLS configuration settings but will not
// populate leaf or ca certficiates.
func (c *Cluster) updateTLSSettingsInConfig(_ AutoConfigOptions, conf *config.Config) error {
	// add in TLS configuration
	if conf.TLS == nil {
		conf.TLS = &config.TLS{}
	}
	conf.TLS.VerifyServerHostname = c.srv.tlsConfigurator.VerifyServerHostname()
	base := c.srv.tlsConfigurator.Base()
	conf.TLS.VerifyOutgoing = base.VerifyOutgoing
	conf.TLS.MinVersion = base.TLSMinVersion
	conf.TLS.PreferServerCipherSuites = base.PreferServerCipherSuites

	var err error
	conf.TLS.CipherSuites, err = tlsutil.CipherString(base.CipherSuites)
	return err
}

// baseConfig will populate the configuration with some base settings such as the
// datacenter names, node name etc.
func (c *Cluster) baseConfig(opts AutoConfigOptions, conf *config.Config) error {
	if opts.NodeName == "" {
		return fmt.Errorf("Cannot generate auto config response without a node name")
	}

	conf.Datacenter = c.srv.config.Datacenter
	conf.PrimaryDatacenter = c.srv.config.PrimaryDatacenter
	conf.NodeName = opts.NodeName
	conf.SegmentName = opts.SegmentName

	return nil
}

type autoConfigUpdater func(c *Cluster, opts AutoConfigOptions, conf *config.Config) error

var (
	// variable holding the list of config updating functions to execute when generating
	// the auto config response. This will allow for more easily adding extra self-contained
	// configurators here in the future.
	autoConfigUpdaters []autoConfigUpdater = []autoConfigUpdater{
		(*Cluster).baseConfig,
		(*Cluster).updateJoinAddressesInConfig,
		(*Cluster).updateGossipEncryptionInConfig,
		(*Cluster).updateTLSSettingsInConfig,
		(*Cluster).updateACLsInConfig,
		(*Cluster).updateTLSCertificatesInConfig,
	}
)

// AgentAutoConfig will authorize the incoming request and then generate the configuration
// to push down to the client
func (c *Cluster) AutoConfig(req *agentpb.AutoConfigRequest, resp *agentpb.AutoConfigResponse) error {
	// default the datacenter to our datacenter - agents do not have to specify this as they may not
	// yet know the datacenter name they are going to be in.
	if req.Datacenter == "" {
		req.Datacenter = c.srv.config.Datacenter
	}

	// TODO (autoconf) Is performing auto configuration over the WAN really a bad idea?
	if req.Datacenter != c.srv.config.Datacenter {
		return fmt.Errorf("invalid datacenter %q - agent auto configuration cannot target a remote datacenter", req.Datacenter)
	}

	// forward to the leader
	if done, err := c.srv.forward("Cluster.AutoConfig", req, req, resp); done {
		return err
	}

	// TODO (autoconf) maybe panic instead?
	if c.authorizer == nil {
		return fmt.Errorf("No Auto Config authorizer is configured")
	}

	// authorize the request with the configured authorizer
	opts, err := c.authorizer.Authorize(req)
	if err != nil {
		return err
	}

	conf := &config.Config{}

	// update all the configurations
	for _, configFn := range autoConfigUpdaters {
		if err := configFn(c, opts, conf); err != nil {
			return err
		}
	}

	resp.Config = conf
	return nil
}
