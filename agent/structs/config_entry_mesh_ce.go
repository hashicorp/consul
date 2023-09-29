//go:build !consulent
// +build !consulent

package structs

func (e *MeshConfigEntry) validateEnterpriseMeta() error {
	return nil
}
