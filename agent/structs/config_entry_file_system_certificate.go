// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
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
	return validateConfigEntryMeta(e.Meta)
}

func (e *FileSystemCertificateConfigEntry) Hosts() ([]string, error) {
	return []string{}, nil
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
