// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import "github.com/hashicorp/consul/agent/config"

// ConfigReloader is a function type which may be implemented to support reloading
// of configuration.
type ConfigReloader func(rtConfig *config.RuntimeConfig) error
