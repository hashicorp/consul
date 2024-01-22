// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package loader handles loading the bootstrap agent config  fetched from HCP into
// the agent's config. It must be a separate package from other HCP components
// because it has a dependency on agent/config while other components need to be
// imported and run within the server process in agent/consul and that would create
// a dependency cycle.
package loader

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/config"
)

type ConfigLoader func(source config.Source) (config.LoadResult, error)

// LoadConfig will attempt to load previously-fetched config from disk and fall back to
// fetch from HCP servers if the local data is incomplete.
// It must be passed a (CLI) UI implementation so it can deliver progress
// updates to the user, for example if it is waiting to retry for a long period.
func LoadConfig(ctx context.Context, client hcpclient.Client, dataDir string, loader ConfigLoader, ui UI) (ConfigLoader, error) {
	ui.Output("Loading configuration from HCP")

	// See if we have existing config on disk
	//
	// OPTIMIZE: We could probably be more intelligent about config loading.
	// The currently implemented approach is:
	// 1. Attempt to load data from disk
	// 2. If that fails or the data is incomplete, block indefinitely fetching remote config.
	//
	// What if instead we had the following flow:
	// 1. Attempt to fetch config from HCP.
	// 2. If that fails, fall back to data on disk from last fetch.
	// 3. If that fails, go into blocking loop to fetch remote config.
	//
	// This should allow us to more gracefully transition cases like when
	// an existing cluster is linked, but then wants to receive TLS materials
	// at a later time. Currently, if we observe the existing-cluster marker we
	// don't attempt to fetch any additional configuration from HCP.

	cfg, ok := loadPersistedBootstrapConfig(dataDir, ui)
	if !ok {
		ui.Info("Fetching configuration from HCP servers")

		var err error
		cfg, err = fetchBootstrapConfig(ctx, client, dataDir, ui)
		if err != nil {
			return nil, fmt.Errorf("failed to bootstrap from HCP: %w", err)
		}
		ui.Info("Configuration fetched from HCP and saved on local disk")

	} else {
		ui.Info("Loaded HCP configuration from local disk")

	}

	// Create a new loader func to return
	newLoader := bootstrapConfigLoader(loader, cfg)
	return newLoader, nil
}

func AddAclPolicyAccessControlHeader(baseLoader ConfigLoader) ConfigLoader {
	return func(source config.Source) (config.LoadResult, error) {
		res, err := baseLoader(source)
		if err != nil {
			return res, err
		}

		rc := res.RuntimeConfig

		// HTTP response headers are modified for the HCP UI to work.
		if rc.HTTPResponseHeaders == nil {
			rc.HTTPResponseHeaders = make(map[string]string)
		}
		prevValue, ok := rc.HTTPResponseHeaders[accessControlHeaderName]
		if !ok {
			rc.HTTPResponseHeaders[accessControlHeaderName] = accessControlHeaderValue
		} else {
			rc.HTTPResponseHeaders[accessControlHeaderName] = prevValue + "," + accessControlHeaderValue
		}

		return res, nil
	}
}

// bootstrapConfigLoader is a ConfigLoader for passing bootstrap JSON config received from HCP
// to the config.builder. ConfigLoaders are functions used to build an agent's RuntimeConfig
// from various sources like files and flags. This config is contained in the config.LoadResult.
//
// The flow to include bootstrap config from HCP as a loader's data source is as follows:
//
//  1. A base ConfigLoader function (baseLoader) is created on agent start, and it sets the input
//     source argument as the DefaultConfig.
//
//  2. When a server agent can be configured by HCP that baseLoader is wrapped in this bootstrapConfigLoader.
//
//  3. The bootstrapConfigLoader calls that base loader with the bootstrap JSON config as the
//     default source. This data will be merged with other valid sources in the config.builder.
//
//  4. The result of the call to baseLoader() below contains the resulting RuntimeConfig, and we do some
//     additional modifications to attach data that doesn't get populated during the build in the config pkg.
//
// Note that since the ConfigJSON is stored as the baseLoader's DefaultConfig, its data is the first
// to be merged by the config.builder and could be overwritten by user-provided values in config files or
// CLI flags. However, values set to RuntimeConfig after the baseLoader call are final.
func bootstrapConfigLoader(baseLoader ConfigLoader, cfg *RawBootstrapConfig) ConfigLoader {
	return func(source config.Source) (config.LoadResult, error) {
		// Don't allow any further attempts to provide a DefaultSource. This should
		// only ever be needed later in client agent AutoConfig code but that should
		// be mutually exclusive from this bootstrapping mechanism since this is
		// only for servers. If we ever try to change that, this clear failure
		// should alert future developers that the assumptions are changing rather
		// than quietly not applying the config they expect!
		if source != nil {
			return config.LoadResult{},
				fmt.Errorf("non-nil config source provided to a loader after HCP bootstrap already provided a DefaultSource")
		}

		// Otherwise, just call to the loader we were passed with our own additional
		// JSON as the source.
		//
		// OPTIMIZE: We could check/log whether any fields set by the remote config were overwritten by a user-provided flag.
		res, err := baseLoader(config.FileSource{
			Name:   "HCP Bootstrap",
			Format: "json",
			Data:   cfg.ConfigJSON,
		})
		if err != nil {
			return res, fmt.Errorf("failed to load HCP Bootstrap config: %w", err)
		}

		finalizeRuntimeConfig(res.RuntimeConfig, cfg)
		return res, nil
	}
}

const (
	accessControlHeaderName  = "Access-Control-Expose-Headers"
	accessControlHeaderValue = "x-consul-default-acl-policy"
)

// finalizeRuntimeConfig will set additional HCP-specific values that are not
// handled by the config.builder.
func finalizeRuntimeConfig(rc *config.RuntimeConfig, cfg *RawBootstrapConfig) {
	rc.Cloud.ManagementToken = cfg.ManagementToken
}

func validatePersistedConfig(dataDir string, filename string) error {
	// Attempt to load persisted config to check for errors and basic validity.
	// Errors here will raise issues like referencing unsupported config fields.
	_, err := config.Load(config.LoadOpts{
		ConfigFiles: []string{filename},
		HCL: []string{
			"server = true",
			`bind_addr = "127.0.0.1"`,
			fmt.Sprintf("data_dir = %q", dataDir),
		},
		ConfigFormat: "json",
	})
	if err != nil {
		return fmt.Errorf("failed to parse local bootstrap config: %w", err)
	}
	return nil
}
