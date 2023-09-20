// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package consul

func (s *Server) enterpriseEvaluateRoleBindings() error {
	return nil
}
