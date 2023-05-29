//go:build !fips

package version

func GetFIPSInfo() string {
	return ""
}
