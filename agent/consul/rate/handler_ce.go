// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package rate

import "github.com/hashicorp/consul/agent/structs"

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

func (h *Handler) UpdateGlobalRateLimitConfig(cfg *structs.GlobalRateLimitConfigEntry) {
}
