//go:build !consulent
// +build !consulent

package ssoauth

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth"
)

func validateType(typ string) error {
	if typ != "jwt" {
		return fmt.Errorf("type should be %q", "jwt")
	}
	return nil
}

func (v *Validator) ssoEntMetaFromClaims(_ *oidcauth.Claims) *structs.EnterpriseMeta {
	return nil
}

type enterpriseConfig struct{}

func (c *Config) enterpriseConvertForLibrary(_ *oidcauth.Config) {}
