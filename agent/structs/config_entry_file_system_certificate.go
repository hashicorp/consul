// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/miekg/dns"

	"github.com/hashicorp/consul/acl"
)

// FileSystemCertificateConfigEntry manages the configuration for a certificate
// and private key located in the local file system.
type FileSystemCertificateConfigEntry struct {
	// Kind of config entry. This will be set to structs.FileSystemCertificate.
	Kind string

	// Name is used to match the config entry with its associated file system certificate.
	Name string

	// Certificate is the optional path to a client certificate to use for TLS connections.
	Certificate string

	// PrivateKey is the optional path to a private key to use for TLS connections.
	PrivateKey string

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

func (e *FileSystemCertificateConfigEntry) SetHash(h uint64) {
	e.Hash = h
}

func (e *FileSystemCertificateConfigEntry) GetHash() uint64 {
	return e.Hash
}

func (e *FileSystemCertificateConfigEntry) GetKind() string { return FileSystemCertificate }
func (e *FileSystemCertificateConfigEntry) GetName() string { return e.Name }
func (e *FileSystemCertificateConfigEntry) Normalize() error {
	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h
	return nil
}
func (e *FileSystemCertificateConfigEntry) GetMeta() map[string]string { return e.Meta }
func (e *FileSystemCertificateConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return &e.EnterpriseMeta
}
func (e *FileSystemCertificateConfigEntry) GetRaftIndex() *RaftIndex { return &e.RaftIndex }

func (e *FileSystemCertificateConfigEntry) Validate() error {
	err := validateConfigEntryMeta(e.Meta)
	if err != nil {
		return err
	}

	privateKeyBlock, _ := pem.Decode([]byte(e.PrivateKey))
	if privateKeyBlock == nil {
		return errors.New("failed to parse private key PEM")
	}

	err = validateKeyLength(privateKeyBlock)
	if err != nil {
		return err
	}

	certificateBlock, _ := pem.Decode([]byte(e.Certificate))
	if certificateBlock == nil {
		return errors.New("failed to parse certificate PEM")
	}

	// make sure we have a valid x509 certificate
	_, err = x509.ParseCertificate(certificateBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// validate that the cert was generated with the given private key
	_, err = tls.X509KeyPair([]byte(e.Certificate), []byte(e.PrivateKey))
	if err != nil {
		return err
	}

	// validate that each host referenced in the CN, DNSSans, and IPSans
	// are valid hostnames
	hosts, err := e.Hosts()
	if err != nil {
		return err
	}
	for _, host := range hosts {
		if _, ok := dns.IsDomainName(host); !ok {
			return fmt.Errorf("host %q must be a valid DNS hostname", host)
		}
	}

	return nil
}

func (e *FileSystemCertificateConfigEntry) Hosts() ([]string, error) {
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

func (e *FileSystemCertificateConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *FileSystemCertificateConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}
