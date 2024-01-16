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
		InsecureSkipVerify: !c.GRPCTLSVerify,
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

type TValue[T string | bool] struct {
	v *T
}

// Set implements the flag.Value interface.
func (t *TValue[T]) Set(v string) error {
	if t.v == nil {
		t.v = new(T)
	}
	var err error
	// have to use interface{}(t.v) to do type assertion
	switch interface{}(t.v).(type) {
	case *string:
		// have to use interface{}(t.v).(*string) to assert t.v as *string
		*(interface{}(t.v).(*string)) = v
	case *bool:
		// have to use interface{}(t.v).(*bool) to assert t.v as *bool
		*(interface{}(t.v).(*bool)), err = strconv.ParseBool(v)
	default:
		err = fmt.Errorf("unsupported type %T", t.v)
	}
	return err
}

// String implements the flag.Value interface.
func (t *TValue[T]) String() string {
	var current T
	if t.v != nil {
		current = *(t.v)
	}
	return fmt.Sprintf("%v", current)
}

// Merge will overlay this value if it has been set.
func (t *TValue[T]) Merge(onto *T) error {
	if onto == nil {
		return fmt.Errorf("onto is nil")
	}
	if t.v != nil {
		*onto = *(t.v)
	}
	return nil
}
