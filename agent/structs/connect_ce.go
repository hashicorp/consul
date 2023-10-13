// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

func (req *ConnectAuthorizeRequest) TargetNamespace() string {
	return IntentionDefaultNamespace
}
