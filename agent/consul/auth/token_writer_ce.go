// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package auth

import "github.com/hashicorp/consul/agent/structs"

func (w *TokenWriter) enterpriseValidation(token, existing *structs.ACLToken) error {
	return nil
}
