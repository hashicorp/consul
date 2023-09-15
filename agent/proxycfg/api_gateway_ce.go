// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package proxycfg

import "context"

func watchJWTProviders(cxt context.Context, h *handlerAPIGateway) error {
	return nil
}

func setJWTProvider(u UpdateEvent, snap *ConfigSnapshot) error {
	return nil
}
