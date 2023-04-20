//go:build !consulent
// +build !consulent

package pbautoconf

func (req *AutoConfigRequest) PartitionOrDefault() string {
	return ""
}
