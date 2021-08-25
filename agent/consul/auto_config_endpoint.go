package consul

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto"

	bexpr "github.com/hashicorp/go-bexpr"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/authmethod/ssoauth"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/template"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/consul/proto/pbconfig"
	"github.com/hashicorp/consul/proto/pbconnect"
	"github.com/hashicorp/consul/tlsutil"
)

type AutoConfigOptions struct {
	NodeName    string
	SegmentName string

	CSR      *x509.CertificateRequest
	SpiffeID *connect.SpiffeIDAgent
}

type AutoConfigAuthorizer interface {
	// Authorizes the request and returns a struct containing the various
	// options for how to generate the configuration.
	Authorize(*pbautoconf.AutoConfigRequest) (AutoConfigOptions, error)
}

type disabledAuthorizer struct{}

func (_ *disabledAuthorizer) Authorize(_ *pbautoconf.AutoConfigRequest) (AutoConfigOptions, error) {
	return AutoConfigOptions{}, fmt.Errorf("Auto Config is disabled")
}

type jwtAuthorizer struct {
	validator       *ssoauth.Validator
	allowReuse      bool
	claimAssertions []string
}

func (a *jwtAuthorizer) Authorize(req *pbautoconf.AutoConfigRequest) (AutoConfigOptions, error) {
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

	opts := AutoConfigOptions{
		NodeName:    req.Node,
		SegmentName: req.Segment,
	}

	if req.CSR != "" {
		csr, id, err := parseAutoConfigCSR(req.CSR)
		if err != nil {
			return AutoConfigOptions{}, err
		}

		if id.Agent != req.Node {
			return AutoConfigOptions{}, fmt.Errorf("Spiffe ID agent name (%s) of the certificate signing request is not for the correct node (%s)", id.Agent, req.Node)
		}

		opts.CSR = csr
		opts.SpiffeID = id
	}

	return opts, nil
}

type AutoConfigBackend interface {
	CreateACLToken(template *structs.ACLToken) (*structs.ACLToken, error)
	DatacenterJoinAddresses(segment string) ([]string, error)
	ForwardRPC(method string, info structs.RPCInfo, reply interface{}) (bool, error)
	GetCARoots() (*structs.IndexedCARoots, error)
	SignCertificate(csr *x509.CertificateRequest, id connect.CertURI) (*structs.IssuedCert, error)
}

// AutoConfig endpoint is used for cluster auto configuration operations
type AutoConfig struct {
	// currently AutoConfig does not support pushing down any configuration that would be reloadable on the servers
	// (outside of some TLS settings such as the configured CA certs which are retrieved via the TLS configurator)
	// If that changes then we will need to change this to use an atomic.Value and provide means of reloading it.
	config          *Config
	tlsConfigurator *tlsutil.Configurator

	backend    AutoConfigBackend
	authorizer AutoConfigAuthorizer
}

func NewAutoConfig(conf *Config, tlsConfigurator *tlsutil.Configurator, backend AutoConfigBackend, authz AutoConfigAuthorizer) *AutoConfig {
	if conf == nil {
		conf = DefaultConfig()
	}

	return &AutoConfig{
		config:          conf,
		tlsConfigurator: tlsConfigurator,
		backend:         backend,
		authorizer:      authz,
	}
}

// updateTLSCertificatesInConfig will ensure that the TLS settings regarding how an agent is
// made aware of its certificates are populated. This will only work if connect is enabled and
// in some cases only if auto_encrypt is enabled on the servers. This endpoint has the option
// to configure auto_encrypt or potentially in the future to generate the certificates inline.
func (ac *AutoConfig) updateTLSCertificatesInConfig(opts AutoConfigOptions, resp *pbautoconf.AutoConfigResponse) error {
	// nothing to be done as we cannot generate certificates
	if !ac.config.ConnectEnabled {
		return nil
	}

	if opts.CSR != nil {
		cert, err := ac.backend.SignCertificate(opts.CSR, opts.SpiffeID)
		if err != nil {
			return fmt.Errorf("Failed to sign CSR: %w", err)
		}

		// convert to the protobuf form of the issued certificate
		pbcert, err := translateIssuedCertToProtobuf(cert)
		if err != nil {
			return err
		}
		resp.Certificate = pbcert
	}

	connectRoots, err := ac.backend.GetCARoots()
	if err != nil {
		return fmt.Errorf("Failed to lookup the CA roots: %w", err)
	}

	// convert to the protobuf form of the issued certificate
	pbroots, err := translateCARootsToProtobuf(connectRoots)
	if err != nil {
		return err
	}

	resp.CARoots = pbroots

	// get the non-connect CA certs from the TLS Configurator
	if ac.tlsConfigurator != nil {
		resp.ExtraCACertificates = ac.tlsConfigurator.ManualCAPems()
	}

	return nil
}

// updateACLtokensInConfig will configure all of the agents ACL settings and will populate
// the configuration with an agent token usable for all default agent operations.
func (ac *AutoConfig) updateACLsInConfig(opts AutoConfigOptions, resp *pbautoconf.AutoConfigResponse) error {
	acl := &pbconfig.ACL{
		Enabled:             ac.config.ACLsEnabled,
		PolicyTTL:           ac.config.ACLResolverSettings.ACLPolicyTTL.String(),
		RoleTTL:             ac.config.ACLResolverSettings.ACLRoleTTL.String(),
		TokenTTL:            ac.config.ACLResolverSettings.ACLTokenTTL.String(),
		DownPolicy:          ac.config.ACLResolverSettings.ACLDownPolicy,
		DefaultPolicy:       ac.config.ACLResolverSettings.ACLDefaultPolicy,
		EnableKeyListPolicy: ac.config.ACLEnableKeyListPolicy,
	}

	// when ACLs are enabled we want to create a local token with a node identity
	if ac.config.ACLsEnabled {
		// set up the token template - the ids and create
		template := structs.ACLToken{
			Description: fmt.Sprintf("Auto Config Token for Node %q", opts.NodeName),
			Local:       true,
			NodeIdentities: []*structs.ACLNodeIdentity{
				{
					NodeName:   opts.NodeName,
					Datacenter: ac.config.Datacenter,
				},
			},
			// TODO(partitions): support auto-config in different partitions
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		}

		token, err := ac.backend.CreateACLToken(&template)
		if err != nil {
			return fmt.Errorf("Failed to generate an ACL token for node %q - %w", opts.NodeName, err)
		}

		acl.Tokens = &pbconfig.ACLTokens{Agent: token.SecretID}
	}

	resp.Config.ACL = acl
	return nil
}

// updateJoinAddressesInConfig determines the correct gossip endpoints that clients should
// be connecting to for joining the cluster based on the segment given in the opts parameter.
func (ac *AutoConfig) updateJoinAddressesInConfig(opts AutoConfigOptions, resp *pbautoconf.AutoConfigResponse) error {
	joinAddrs, err := ac.backend.DatacenterJoinAddresses(opts.SegmentName)
	if err != nil {
		return err
	}

	if resp.Config.Gossip == nil {
		resp.Config.Gossip = &pbconfig.Gossip{}
	}

	resp.Config.Gossip.RetryJoinLAN = joinAddrs
	return nil
}

// updateGossipEncryptionInConfig will populate the gossip encryption configuration settings
func (ac *AutoConfig) updateGossipEncryptionInConfig(_ AutoConfigOptions, resp *pbautoconf.AutoConfigResponse) error {
	// Add gossip encryption settings if there is any key loaded
	memberlistConfig := ac.config.SerfLANConfig.MemberlistConfig
	if lanKeyring := memberlistConfig.Keyring; lanKeyring != nil {
		if resp.Config.Gossip == nil {
			resp.Config.Gossip = &pbconfig.Gossip{}
		}
		if resp.Config.Gossip.Encryption == nil {
			resp.Config.Gossip.Encryption = &pbconfig.GossipEncryption{}
		}

		pk := lanKeyring.GetPrimaryKey()
		if len(pk) > 0 {
			resp.Config.Gossip.Encryption.Key = base64.StdEncoding.EncodeToString(pk)
		}

		resp.Config.Gossip.Encryption.VerifyIncoming = memberlistConfig.GossipVerifyIncoming
		resp.Config.Gossip.Encryption.VerifyOutgoing = memberlistConfig.GossipVerifyOutgoing
	}

	return nil
}

// updateTLSSettingsInConfig will populate the TLS configuration settings but will not
// populate leaf or ca certficiates.
func (ac *AutoConfig) updateTLSSettingsInConfig(_ AutoConfigOptions, resp *pbautoconf.AutoConfigResponse) error {
	if ac.tlsConfigurator == nil {
		// TLS is not enabled?
		return nil
	}

	// add in TLS configuration
	if resp.Config.TLS == nil {
		resp.Config.TLS = &pbconfig.TLS{}
	}

	resp.Config.TLS.VerifyServerHostname = ac.tlsConfigurator.VerifyServerHostname()
	base := ac.tlsConfigurator.Base()
	resp.Config.TLS.VerifyOutgoing = base.VerifyOutgoing
	resp.Config.TLS.MinVersion = base.TLSMinVersion
	resp.Config.TLS.PreferServerCipherSuites = base.PreferServerCipherSuites

	var err error
	resp.Config.TLS.CipherSuites, err = tlsutil.CipherString(base.CipherSuites)
	return err
}

// baseConfig will populate the configuration with some base settings such as the
// datacenter names, node name etc.
func (ac *AutoConfig) baseConfig(opts AutoConfigOptions, resp *pbautoconf.AutoConfigResponse) error {
	if opts.NodeName == "" {
		return fmt.Errorf("Cannot generate auto config response without a node name")
	}

	resp.Config.Datacenter = ac.config.Datacenter
	resp.Config.PrimaryDatacenter = ac.config.PrimaryDatacenter
	resp.Config.NodeName = opts.NodeName
	resp.Config.SegmentName = opts.SegmentName

	return nil
}

type autoConfigUpdater func(c *AutoConfig, opts AutoConfigOptions, resp *pbautoconf.AutoConfigResponse) error

var (
	// variable holding the list of config updating functions to execute when generating
	// the auto config response. This will allow for more easily adding extra self-contained
	// configurators here in the future.
	autoConfigUpdaters []autoConfigUpdater = []autoConfigUpdater{
		(*AutoConfig).baseConfig,
		(*AutoConfig).updateJoinAddressesInConfig,
		(*AutoConfig).updateGossipEncryptionInConfig,
		(*AutoConfig).updateTLSSettingsInConfig,
		(*AutoConfig).updateACLsInConfig,
		(*AutoConfig).updateTLSCertificatesInConfig,
	}
)

// AgentAutoConfig will authorize the incoming request and then generate the configuration
// to push down to the client
func (ac *AutoConfig) InitialConfiguration(req *pbautoconf.AutoConfigRequest, resp *pbautoconf.AutoConfigResponse) error {
	// default the datacenter to our datacenter - agents do not have to specify this as they may not
	// yet know the datacenter name they are going to be in.
	if req.Datacenter == "" {
		req.Datacenter = ac.config.Datacenter
	}

	// TODO (autoconf) Is performing auto configuration over the WAN really a bad idea?
	if req.Datacenter != ac.config.Datacenter {
		return fmt.Errorf("invalid datacenter %q - agent auto configuration cannot target a remote datacenter", req.Datacenter)
	}

	// TODO (autoconf) maybe panic instead?
	if ac.backend == nil {
		return fmt.Errorf("No Auto Config backend is configured")
	}

	// forward to the leader
	if done, err := ac.backend.ForwardRPC("AutoConfig.InitialConfiguration", req, resp); done {
		return err
	}

	// TODO (autoconf) maybe panic instead?
	if ac.authorizer == nil {
		return fmt.Errorf("No Auto Config authorizer is configured")
	}

	// authorize the request with the configured authorizer
	opts, err := ac.authorizer.Authorize(req)
	if err != nil {
		return err
	}

	resp.Config = &pbconfig.Config{}

	// update all the configurations
	for _, configFn := range autoConfigUpdaters {
		if err := configFn(ac, opts, resp); err != nil {
			return err
		}
	}

	return nil
}

func parseAutoConfigCSR(csr string) (*x509.CertificateRequest, *connect.SpiffeIDAgent, error) {
	// Parse the CSR string into the x509 CertificateRequest struct
	x509CSR, err := connect.ParseCSR(csr)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse CSR: %w", err)
	}

	// ensure that a URI SAN is present
	if len(x509CSR.URIs) < 1 {
		return nil, nil, fmt.Errorf("CSR didn't include any URI SANs")
	}

	// Parse the SPIFFE ID
	spiffeID, err := connect.ParseCertURI(x509CSR.URIs[0])
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse the SPIFFE URI: %w", err)
	}

	agentID, isAgent := spiffeID.(*connect.SpiffeIDAgent)
	if !isAgent {
		return nil, nil, fmt.Errorf("SPIFFE ID is not an Agent ID")
	}

	return x509CSR, agentID, nil
}

func translateCARootsToProtobuf(in *structs.IndexedCARoots) (*pbconnect.CARoots, error) {
	var out pbconnect.CARoots
	if err := mapstructureTranslateToProtobuf(in, &out); err != nil {
		return nil, fmt.Errorf("Failed to re-encode CA Roots: %w", err)
	}

	return &out, nil
}

func translateIssuedCertToProtobuf(in *structs.IssuedCert) (*pbconnect.IssuedCert, error) {
	var out pbconnect.IssuedCert
	if err := mapstructureTranslateToProtobuf(in, &out); err != nil {
		return nil, fmt.Errorf("Failed to re-encode CA Roots: %w", err)
	}

	return &out, nil
}

func mapstructureTranslateToProtobuf(in interface{}, out interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: proto.HookTimeToPBTimestamp,
		Result:     out,
	})

	if err != nil {
		return err
	}

	return decoder.Decode(in)
}
