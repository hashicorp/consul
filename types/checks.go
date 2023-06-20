// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

// CheckID is a strongly typed string used to uniquely represent a Consul
// Check on an Agent (a CheckID is not globally unique).
type CheckID string
