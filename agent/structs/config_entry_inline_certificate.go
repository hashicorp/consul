package structs

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// InlineCertificateConfigEntry manages the configuration for an inline certificate
// with the given name.
type InlineCertificateConfigEntry struct {
	// Kind of config entry. This will be set to structs.InlineCertificate.
	Kind string

	// Name is used to match the config entry with its associated inline certificate.
	Name string

	// Certificate is the public certificate component of an x509 key pair encoded in raw PEM format.
	Certificate string
	// PrivateKey is the private key component of an x509 key pair encoded in raw PEM format.
	PrivateKey string

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *InlineCertificateConfigEntry) GetKind() string            { return InlineCertificate }
func (e *InlineCertificateConfigEntry) GetName() string            { return e.Name }
func (e *InlineCertificateConfigEntry) Normalize() error           { return nil }
func (e *InlineCertificateConfigEntry) GetMeta() map[string]string { return e.Meta }
func (e *InlineCertificateConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return &e.EnterpriseMeta
}
func (e *InlineCertificateConfigEntry) GetRaftIndex() *RaftIndex { return &e.RaftIndex }

func (e *InlineCertificateConfigEntry) Validate() error {
	privateKeyBlock, _ := pem.Decode([]byte(e.PrivateKey))
	if privateKeyBlock == nil {
		return errors.New("failed to parse private key PEM")
	}

	certificateBlock, _ := pem.Decode([]byte(e.Certificate))
	if certificateBlock == nil {
		return errors.New("failed to parse certificate PEM")
	}

	// make sure we have a valid x509 certificate
	_, err := x509.ParseCertificate(certificateBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// validate that the cert was generated with the given private key
	_, err = tls.X509KeyPair([]byte(e.Certificate), []byte(e.PrivateKey))
	if err != nil {
		return err
	}

	return nil
}

func (e *InlineCertificateConfigEntry) Hosts() ([]string, error) {
	certificateBlock, _ := pem.Decode([]byte(e.Certificate))
	if certificateBlock == nil {
		return nil, errors.New("failed to parse certificate PEM")
	}

	certificate, err := x509.ParseCertificate(certificateBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	hosts := []string{certificate.Subject.CommonName}

	for _, name := range certificate.DNSNames {
		hosts = append(hosts, name)
	}

	for _, ip := range certificate.IPAddresses {
		hosts = append(hosts, ip.String())
	}

	return hosts, nil
}

func (e *InlineCertificateConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *InlineCertificateConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}
