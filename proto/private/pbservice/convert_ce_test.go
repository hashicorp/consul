// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package pbservice

import (
	fuzz "github.com/google/gofuzz"

	"github.com/hashicorp/consul/acl"
)

func randEnterpriseMeta(_ *acl.EnterpriseMeta, _ fuzz.Continue) {
}
