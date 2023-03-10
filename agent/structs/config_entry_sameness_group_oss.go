//go:build !consulent
// +build !consulent

package structs

import "fmt"

func (s *SamenessGroupConfigEntry) Validate() error {
	return fmt.Errorf("sameness-groups are an enterprise-only feature")
}
