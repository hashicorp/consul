// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configfetcher

// ConfigFetcher is the interface the agent needs to expose
// for the xDS server to fetch agent config, currently only one field is fetched
type ConfigFetcher interface {
	AdvertiseAddrLAN() string
}
