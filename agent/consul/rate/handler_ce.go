// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package rate

type IPLimitConfig struct{}

func (h *Handler) UpdateIPConfig(cfg IPLimitConfig) {
	// noop
}

func (h *Handler) ipGlobalLimit(op Operation) *limit {
	return nil
}

func (h *Handler) ipCategoryLimit(op Operation) *limit {
	return nil
}
