//go:build !consulent
// +build !consulent

package cachetype

func (req *ConnectCALeafRequest) TargetNamespace() string {
	return "default"
}
