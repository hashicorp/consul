//go:build !consulent
// +build !consulent

package structs

func (c *AutopilotConfig) autopilotConfigExt() interface{} {
	return nil
}
