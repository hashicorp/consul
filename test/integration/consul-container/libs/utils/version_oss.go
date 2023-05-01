//go:build !consulent
// +build !consulent

package utils

const (
	DefaultImageName   = "consul"
)

func ImageName(image, version string) string {
	return image + ":" + version
}
