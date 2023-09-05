// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package sentinel

import (
	"github.com/hashicorp/go-hclog"
)

// New returns a new instance of the Sentinel code engine. This is only available
// in Consul Enterprise so this version always returns nil.
func New(logger hclog.Logger) Evaluator {
	return nil
}
