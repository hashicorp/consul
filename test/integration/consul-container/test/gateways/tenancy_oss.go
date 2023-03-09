//go:build !consulent
// +build !consulent

package gateways

func getNamespace() string {
	return ""
}

func getPartition() string {
	return ""
}
