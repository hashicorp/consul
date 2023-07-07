//go:build !fips

package version

func IsFIPS() bool {
	return false
}

func GetFIPSInfo() string {
	return ""
}
