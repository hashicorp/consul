// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package consul

func (s *Server) enterpriseEvaluateRoleBindings() error {
	return nil
}
