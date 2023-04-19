//go:build !consulent
// +build !consulent

package rate

type IPLimitConfig struct {
}

func (h *Handler) UpdateIPConfig(cfg IPLimitConfig) {
	// noop
}
