// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package types

// CheckID is a strongly typed string used to uniquely represent a Consul
// Check on an Agent (a CheckID is not globally unique).
type CheckID string
