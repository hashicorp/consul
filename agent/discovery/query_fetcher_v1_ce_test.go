// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package discovery

import "github.com/hashicorp/consul/acl"

// defaultEntMeta is the default enterprise meta used for testing.
var defaultEntMeta = acl.EnterpriseMeta{}
