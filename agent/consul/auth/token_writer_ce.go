//go:build !consulent
// +build !consulent

package auth

import "github.com/hashicorp/consul/agent/structs"

func (w *TokenWriter) enterpriseValidation(token, existing *structs.ACLToken) error {
	return nil
}
