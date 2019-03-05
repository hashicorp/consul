package plugin

import (
	"crypto/x509"
	"net/rpc"

	"github.com/hashicorp/consul/agent/connect/ca"
)

// providerPluginRPCServer implements a net/rpc backed transport for
// an underlying implementation of a ca.Provider. The server side is the
// plugin binary itself.
type providerPluginRPCServer struct {
	impl ca.Provider
}

func (p *providerPluginRPCServer) Configure(args *ConfigureRPCRequest, _ *struct{}) error {
	return p.impl.Configure(args.ClusterId, args.IsRoot, args.RawConfig)
}

func (p *providerPluginRPCServer) GenerateRoot(struct{}, *struct{}) error {
	return p.impl.GenerateRoot()
}

func (p *providerPluginRPCServer) ActiveRoot(_ struct{}, resp *ActiveRootResponse) error {
	var err error
	resp.CrtPem, err = p.impl.ActiveRoot()
	return err
}

func (p *providerPluginRPCServer) GenerateIntermediateCSR(_ struct{}, resp *GenerateIntermediateCSRResponse) error {
	var err error
	resp.CsrPem, err = p.impl.GenerateIntermediateCSR()
	return err
}

func (p *providerPluginRPCServer) SetIntermediate(args *SetIntermediateRPCRequest, _ *struct{}) error {
	return p.impl.SetIntermediate(args.IntermediatePEM, args.RootPEM)
}

func (p *providerPluginRPCServer) ActiveIntermediate(_ struct{}, resp *ActiveIntermediateResponse) error {
	var err error
	resp.CrtPem, err = p.impl.ActiveIntermediate()
	return err
}

func (p *providerPluginRPCServer) GenerateIntermediate(_ struct{}, resp *GenerateIntermediateResponse) error {
	var err error
	resp.CrtPem, err = p.impl.GenerateIntermediate()
	return err
}

func (p *providerPluginRPCServer) Sign(args *SignRequest, resp *SignResponse) error {
	csr, err := x509.ParseCertificateRequest(args.Csr)
	if err != nil {
		return err
	}

	resp.CrtPem, err = p.impl.Sign(csr)
	return err
}

func (p *providerPluginRPCServer) SignIntermediate(args *SignIntermediateRequest, resp *SignIntermediateResponse) error {
	csr, err := x509.ParseCertificateRequest(args.Csr)
	if err != nil {
		return err
	}

	resp.CrtPem, err = p.impl.SignIntermediate(csr)
	return err
}

func (p *providerPluginRPCServer) CrossSignCA(args *CrossSignCARequest, resp *CrossSignCAResponse) error {
	crt, err := x509.ParseCertificate(args.Crt)
	if err != nil {
		return err
	}

	resp.CrtPem, err = p.impl.CrossSignCA(crt)
	return err
}

func (p *providerPluginRPCServer) Cleanup(struct{}, *struct{}) error {
	return p.impl.Cleanup()
}

// providerPluginRPCClient implements a net/rpc backed transport for
// an underlying implementation of a ca.Provider. The client side is the
// software calling into the plugin binary over rpc.
//
// This implements ca.Provider.
type providerPluginRPCClient struct {
	client *rpc.Client
}

func (p *providerPluginRPCClient) Configure(
	clusterId string,
	isRoot bool,
	rawConfig map[string]interface{}) error {
	return p.client.Call("Plugin.Configure", &ConfigureRPCRequest{
		ClusterId: clusterId,
		IsRoot:    isRoot,
		RawConfig: rawConfig,
	}, &struct{}{})
}

func (p *providerPluginRPCClient) GenerateRoot() error {
	return p.client.Call("Plugin.GenerateRoot", struct{}{}, &struct{}{})
}

func (p *providerPluginRPCClient) ActiveRoot() (string, error) {
	var resp ActiveRootResponse
	err := p.client.Call("Plugin.ActiveRoot", struct{}{}, &resp)
	return resp.CrtPem, err
}

func (p *providerPluginRPCClient) GenerateIntermediateCSR() (string, error) {
	var resp GenerateIntermediateCSRResponse
	err := p.client.Call("Plugin.GenerateIntermediateCSR", struct{}{}, &resp)
	return resp.CsrPem, err
}

func (p *providerPluginRPCClient) SetIntermediate(intermediatePEM, rootPEM string) error {
	return p.client.Call("Plugin.SetIntermediate", &SetIntermediateRPCRequest{
		IntermediatePEM: intermediatePEM,
		RootPEM:         rootPEM,
	}, &struct{}{})
}

func (p *providerPluginRPCClient) ActiveIntermediate() (string, error) {
	var resp ActiveIntermediateResponse
	err := p.client.Call("Plugin.ActiveIntermediate", struct{}{}, &resp)
	return resp.CrtPem, err
}

func (p *providerPluginRPCClient) GenerateIntermediate() (string, error) {
	var resp GenerateIntermediateResponse
	err := p.client.Call("Plugin.GenerateIntermediate", struct{}{}, &resp)
	return resp.CrtPem, err
}

func (p *providerPluginRPCClient) Sign(csr *x509.CertificateRequest) (string, error) {
	var resp SignResponse
	err := p.client.Call("Plugin.Sign", &SignRequest{
		Csr: csr.Raw,
	}, &resp)
	return resp.CrtPem, err
}

func (p *providerPluginRPCClient) SignIntermediate(csr *x509.CertificateRequest) (string, error) {
	var resp SignIntermediateResponse
	err := p.client.Call("Plugin.SignIntermediate", &SignIntermediateRequest{
		Csr: csr.Raw,
	}, &resp)
	return resp.CrtPem, err
}

func (p *providerPluginRPCClient) CrossSignCA(crt *x509.Certificate) (string, error) {
	var resp CrossSignCAResponse
	err := p.client.Call("Plugin.CrossSignCA", &CrossSignCARequest{
		Crt: crt.Raw,
	}, &resp)
	return resp.CrtPem, err
}

func (p *providerPluginRPCClient) Cleanup() error {
	return p.client.Call("Plugin.Cleanup", struct{}{}, &struct{}{})
}

// Verification
var _ ca.Provider = &providerPluginRPCClient{}

//-------------------------------------------------------------------
// Structs for net/rpc request and response

type ConfigureRPCRequest struct {
	ClusterId string
	IsRoot    bool
	RawConfig map[string]interface{}
}

type SetIntermediateRPCRequest struct {
	IntermediatePEM string
	RootPEM         string
}
