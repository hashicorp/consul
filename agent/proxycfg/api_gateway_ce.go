// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package proxycfg

import "context"

func watchJWTProviders(cxt context.Context, h *handlerAPIGateway) error {
	return nil
}

func setJWTProvider(u UpdateEvent, snap *ConfigSnapshot) error {
	return nil
}
