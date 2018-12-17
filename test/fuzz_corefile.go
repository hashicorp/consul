// +build fuzz

package test

// Fuzz fuzzes a corefile.
func Fuzz(data []byte) int {
	_, _, _, err := CoreDNSServerAndPorts(string(data))
	if err != nil {
		return 1
	}
	return 0
}
