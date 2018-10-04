package plugin

import (
	"context"
	"crypto/x509"
	"encoding/json"

	"github.com/hashicorp/consul/agent/connect/ca"
	"google.golang.org/grpc"
)

// providerPluginGRPCServer implements the CAServer interface for gRPC.
type providerPluginGRPCServer struct {
	impl ca.Provider
}

func (p *providerPluginGRPCServer) Configure(_ context.Context, req *ConfigureRequest) (*Empty, error) {
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(req.Config, &rawConfig); err != nil {
		return nil, err
	}

	return &Empty{}, p.impl.Configure(req.ClusterId, req.IsRoot, rawConfig)
}

func (p *providerPluginGRPCServer) GenerateRoot(context.Context, *Empty) (*Empty, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) ActiveRoot(context.Context, *Empty) (*ActiveRootResponse, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) GenerateIntermediateCSR(context.Context, *Empty) (*GenerateIntermediateCSRResponse, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) SetIntermediate(context.Context, *SetIntermediateRequest) (*Empty, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) ActiveIntermediate(context.Context, *Empty) (*ActiveIntermediateResponse, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) GenerateIntermediate(context.Context, *Empty) (*GenerateIntermediateResponse, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) Sign(context.Context, *SignRequest) (*SignResponse, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) SignIntermediate(context.Context, *SignIntermediateRequest) (*SignIntermediateResponse, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) CrossSignCA(context.Context, *CrossSignCARequest) (*CrossSignCAResponse, error) {
	return nil, nil
}

func (p *providerPluginGRPCServer) Cleanup(context.Context, *Empty) (*Empty, error) {
	return &Empty{}, p.impl.Cleanup()
}

// providerPluginGRPCClient implements ca.Provider for acting as a client
// to a remote CA provider plugin over gRPC.
type providerPluginGRPCClient struct {
	client     CAClient
	clientConn *grpc.ClientConn
	doneCtx    context.Context
}

func (p *providerPluginGRPCClient) Configure(
	clusterId string,
	isRoot bool,
	rawConfig map[string]interface{}) error {
	config, err := json.Marshal(rawConfig)
	if err != nil {
		return err
	}

	_, err = p.client.Configure(p.doneCtx, &ConfigureRequest{
		ClusterId: clusterId,
		IsRoot:    isRoot,
		Config:    config,
	})
	return p.err(err)
}

func (p *providerPluginGRPCClient) GenerateRoot() error {
	_, err := p.client.GenerateRoot(p.doneCtx, &Empty{})
	return p.err(err)
}

func (p *providerPluginGRPCClient) ActiveRoot() (string, error) {
	resp, err := p.client.ActiveRoot(p.doneCtx, &Empty{})
	if err != nil {
		return "", p.err(err)
	}

	return resp.CrtPem, nil
}

func (p *providerPluginGRPCClient) GenerateIntermediateCSR() (string, error) {
	resp, err := p.client.GenerateIntermediateCSR(p.doneCtx, &Empty{})
	if err != nil {
		return "", p.err(err)
	}

	return resp.CsrPem, nil
}

func (p *providerPluginGRPCClient) SetIntermediate(intermediatePEM, rootPEM string) error {
	_, err := p.client.SetIntermediate(p.doneCtx, &SetIntermediateRequest{
		IntermediatePem: intermediatePEM,
		RootPem:         rootPEM,
	})
	return p.err(err)
}

func (p *providerPluginGRPCClient) ActiveIntermediate() (string, error) {
	resp, err := p.client.ActiveIntermediate(p.doneCtx, &Empty{})
	if err != nil {
		return "", p.err(err)
	}

	return resp.CrtPem, nil
}

func (p *providerPluginGRPCClient) GenerateIntermediate() (string, error) {
	resp, err := p.client.GenerateIntermediate(p.doneCtx, &Empty{})
	if err != nil {
		return "", p.err(err)
	}

	return resp.CrtPem, nil
}

func (p *providerPluginGRPCClient) Sign(*x509.CertificateRequest) (string, error) {
	// TODO(mitchellh)
	return "", nil
}

func (p *providerPluginGRPCClient) SignIntermediate(*x509.CertificateRequest) (string, error) {
	// TODO(mitchellh)
	return "", nil
}

func (p *providerPluginGRPCClient) CrossSignCA(*x509.Certificate) (string, error) {
	// TODO(mitchellh)
	return "", nil
}

func (p *providerPluginGRPCClient) Cleanup() error {
	_, err := p.client.Cleanup(p.doneCtx, &Empty{})
	return p.err(err)
}

func (p *providerPluginGRPCClient) err(err error) error {
	if err := p.doneCtx.Err(); err != nil {
		return err
	}

	return err
}

// Verification
var _ CAServer = &providerPluginGRPCServer{}
var _ ca.Provider = &providerPluginGRPCClient{}
