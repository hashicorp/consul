// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package proxystateconverter

import (
	"fmt"
)

func (ft discoChainTargets) orderByLocality() ([]discoChainTargetGroup, error) {
	return nil, fmt.Errorf("order-by-locality is a Consul Enterprise feature")
}
