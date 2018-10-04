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
	return &Empty{}, p.impl.GenerateRoot()
}

func (p *providerPluginGRPCServer) ActiveRoot(context.Context, *Empty) (*ActiveRootResponse, error) {
	pem, err := p.impl.ActiveRoot()
	return &ActiveRootResponse{CrtPem: pem}, err
}

func (p *providerPluginGRPCServer) GenerateIntermediateCSR(context.Context, *Empty) (*GenerateIntermediateCSRResponse, error) {
	pem, err := p.impl.GenerateIntermediateCSR()
	return &GenerateIntermediateCSRResponse{CsrPem: pem}, err
}

func (p *providerPluginGRPCServer) SetIntermediate(_ context.Context, req *SetIntermediateRequest) (*Empty, error) {
	return &Empty{}, p.impl.SetIntermediate(req.IntermediatePem, req.RootPem)
}

func (p *providerPluginGRPCServer) ActiveIntermediate(context.Context, *Empty) (*ActiveIntermediateResponse, error) {
	pem, err := p.impl.ActiveIntermediate()
	return &ActiveIntermediateResponse{CrtPem: pem}, err
}

func (p *providerPluginGRPCServer) GenerateIntermediate(context.Context, *Empty) (*GenerateIntermediateResponse, error) {
	pem, err := p.impl.GenerateIntermediate()
	return &GenerateIntermediateResponse{CrtPem: pem}, err
}

func (p *providerPluginGRPCServer) Sign(_ context.Context, req *SignRequest) (*SignResponse, error) {
	csr, err := x509.ParseCertificateRequest(req.Csr)
	if err != nil {
		return nil, err
	}

	crtPEM, err := p.impl.Sign(csr)
	return &SignResponse{CrtPem: crtPEM}, err
}

func (p *providerPluginGRPCServer) SignIntermediate(_ context.Context, req *SignIntermediateRequest) (*SignIntermediateResponse, error) {
	csr, err := x509.ParseCertificateRequest(req.Csr)
	if err != nil {
		return nil, err
	}

	crtPEM, err := p.impl.SignIntermediate(csr)
	return &SignIntermediateResponse{CrtPem: crtPEM}, err
}

func (p *providerPluginGRPCServer) CrossSignCA(_ context.Context, req *CrossSignCARequest) (*CrossSignCAResponse, error) {
	crt, err := x509.ParseCertificate(req.Crt)
	if err != nil {
		return nil, err
	}

	crtPEM, err := p.impl.CrossSignCA(crt)
	return &CrossSignCAResponse{CrtPem: crtPEM}, err
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

func (p *providerPluginGRPCClient) Sign(csr *x509.CertificateRequest) (string, error) {
	resp, err := p.client.Sign(p.doneCtx, &SignRequest{
		Csr: csr.Raw,
	})
	if err != nil {
		return "", p.err(err)
	}

	return resp.CrtPem, nil
}

func (p *providerPluginGRPCClient) SignIntermediate(csr *x509.CertificateRequest) (string, error) {
	resp, err := p.client.SignIntermediate(p.doneCtx, &SignIntermediateRequest{
		Csr: csr.Raw,
	})
	if err != nil {
		return "", p.err(err)
	}

	return resp.CrtPem, nil
}

func (p *providerPluginGRPCClient) CrossSignCA(crt *x509.Certificate) (string, error) {
	resp, err := p.client.CrossSignCA(p.doneCtx, &CrossSignCARequest{
		Crt: crt.Raw,
	})
	if err != nil {
		return "", p.err(err)
	}

	return resp.CrtPem, nil
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
