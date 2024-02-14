package proxytarget

import "fmt"

func GetAlpnProtocolFromPortName(portName string) string {
	return fmt.Sprintf("consul~%s", portName)
}
