// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/go-rootcerts"
)

// tls.Config is used to establish communication in TLS mode
func SetupTLSConfig(c *GRPCConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !c.GRPCSSLVerifyEnvName,
	}

	if c.CertFile != "" && c.KeyFile != "" {
		tlsCert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	var caConfig *rootcerts.Config
	if c.CAFile != "" || c.CAPath != "" {
		caConfig = &rootcerts.Config{
			CAFile: c.CAFile,
			CAPath: c.CAPath,
		}
	}
	// load system CA certs if user doesn't provide any
	if err := rootcerts.ConfigureTLS(tlsConfig, caConfig); err != nil {
		return nil, err
	}

	return tlsConfig, nil
}

func removeSchemaFromGRPCAddress(addr string) string {
	// Parse as host:port with option http prefix
	grpcAddr := strings.TrimPrefix(addr, "http://")
	grpcAddr = strings.TrimPrefix(grpcAddr, "https://")
	return grpcAddr
}

type StringValue struct {
	v *string
}

// Set implements the flag.Value interface.
func (s *StringValue) Set(v string) error {
	if s.v == nil {
		s.v = new(string)
	}
	*(s.v) = v
	return nil
}

// String implements the flag.Value interface.
func (s *StringValue) String() string {
	var current string
	if s.v != nil {
		current = *(s.v)
	}
	return current
}

// Merge will overlay this value if it has been set.
func (s *StringValue) Merge(onto *string) {
	if s.v != nil {
		*onto = *(s.v)
	}
}

// BoolValue provides a flag value that's aware if it has been set.
type BoolValue struct {
	v *bool
}

// Set implements the flag.Value interface.
func (b *BoolValue) Set(v string) error {
	if b.v == nil {
		b.v = new(bool)
	}
	var err error
	*(b.v), err = strconv.ParseBool(v)
	return err
}

// String implements the flag.Value interface.
func (b *BoolValue) String() string {
	var current bool
	if b.v != nil {
		current = *(b.v)
	}
	return fmt.Sprintf("%v", current)
}

// Merge will overlay this value if it has been set.
func (b *BoolValue) Merge(onto *bool) {
	if b.v != nil {
		*onto = *(b.v)
	}
}
