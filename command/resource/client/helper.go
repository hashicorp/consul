// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/api"
)

// SetupTLSConfig returns the TLS configuration used to communicate with Consul
// over gRPC with TLS. It shares the HTTP client's TLS setup, so the server's
// certificate is verified unless verification was explicitly disabled.
func SetupTLSConfig(c *GRPCConfig) (*tls.Config, error) {
	tlsConfig, err := api.SetupTLSConfig(&api.TLSConfig{
		CAFile:             c.CAFile,
		CAPath:             c.CAPath,
		CertFile:           c.CertFile,
		KeyFile:            c.KeyFile,
		InsecureSkipVerify: c.InsecureSkipVerify,
	})
	if err != nil {
		return nil, err
	}
	tlsConfig.ServerName = tlsServerName(c.TLSServerName)
	tlsConfig.MinVersion = tls.VersionTLS12

	return tlsConfig, nil
}

// tlsServerName returns the host to verify the server's certificate against.
// It accepts a host or IP address with or without a port, including
// unbracketed IPv6 addresses, which net.SplitHostPort rejects.
func tlsServerName(name string) string {
	if host, _, err := net.SplitHostPort(name); err == nil {
		return host
	}
	return strings.TrimSuffix(strings.TrimPrefix(name, "["), "]")
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
