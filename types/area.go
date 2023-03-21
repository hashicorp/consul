// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

// AreaID is a strongly-typed string used to uniquely represent a network area,
// which is a relationship between Consul servers.
type AreaID string

// This represents the existing WAN area that's built in to Consul. Consul
// Enterprise generalizes areas, which are represented with UUIDs.
const AreaWAN AreaID = "wan"

// This represents the existing LAN area that's built in to Consul. Consul
// Enterprise generalizes areas, which are represented with UUIDs.
const AreaLAN AreaID = "lan"
