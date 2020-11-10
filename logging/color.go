package logging

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
)

func NewColorOption(v string) (hclog.ColorOption, error) {
	switch v {
	case "", "auto":
		return hclog.AutoColor, nil
	case "always", "on", "enabled":
		return hclog.ForceColor, nil
	case "never", "off", "disabled":
		return hclog.ColorOff, nil
	default:
		return hclog.ColorOff, fmt.Errorf("invalid color value %v, must be one of: auto,on,off", v)
	}
}
