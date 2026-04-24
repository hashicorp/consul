// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

func virtualIPForServicePort(_ *state.Store, _ structs.PeeredServiceName, _ string) (string, bool, error) {
	return "", false, nil
}
