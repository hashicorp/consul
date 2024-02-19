// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package types

import (
	"errors"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

func validateAction(data interface{ GetAction() pbauth.Action }) error {
	switch data.GetAction() {
	case pbauth.Action_ACTION_ALLOW:
	default:
		return resource.ErrInvalidField{
			Name:    "data.action",
			Wrapped: errors.New("action must be allow"),
		}
	}
	return nil
}
