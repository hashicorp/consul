// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package resource

import pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"

func blockBuiltinsDeletion(rtype *pbresource.Type, id *pbresource.ID) error {
	if err := blockDefaultNamespaceDeletion(rtype, id); err != nil {
		return err
	}
	return nil
}
