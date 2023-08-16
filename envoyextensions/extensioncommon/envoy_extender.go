// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package extensioncommon

import (
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
)

// EnvoyExtender is the interface that all Envoy extensions must implement in order
// to be dynamically executed during runtime.
type EnvoyExtender interface {

	// CanApply checks whether the extension configured for this extender is eligible
	// for application based on the specified RuntimeConfig.
	CanApply(*RuntimeConfig) bool

	// Validate ensures the data in config can successfuly be used
	// to apply the specified Envoy extension.
	Validate(*RuntimeConfig) error

	// Extend updates indexed xDS structures to include patches for
	// built-in extensions. It is responsible for applying extensions to
	// the appropriate xDS resources. If any portion of this function fails,
	// it will attempt continue and return an error. The caller can then determine
	// if it is better to use a partially applied extension or error out.
	Extend(*xdscommon.IndexedResources, *RuntimeConfig) (*xdscommon.IndexedResources, error)
}
