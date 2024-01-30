// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package types

import (
	"errors"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

var v validator = &actionValidator{}

type actionValidator struct{}

func (v *actionValidator) ValidateAction(data interface{ GetAction() pbauth.Action }) error {
	// enumcover:pbauth.Action
	switch data.GetAction() {
	case pbauth.Action_ACTION_ALLOW:
	case pbauth.Action_ACTION_UNSPECIFIED:
		fallthrough
	case pbauth.Action_ACTION_DENY:
		fallthrough
	default:
		return resource.ErrInvalidField{
			Name:    "data.action",
			Wrapped: errors.New("action must be allow"),
		}
	}
	return nil
}

var _ validator = (*actionValidator)(nil)
