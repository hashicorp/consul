//go:build !consulent
// +build !consulent

package consul

func (s *Server) enterpriseEvaluateRoleBindings() error {
	return nil
}
