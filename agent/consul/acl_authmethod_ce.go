// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

func (s *Server) enterpriseEvaluateRoleBindings() error {
	return nil
}
