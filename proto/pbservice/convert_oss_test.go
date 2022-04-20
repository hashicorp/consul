//go:build !consulent
// +build !consulent

package pbservice

import (
	fuzz "github.com/google/gofuzz"

	"github.com/hashicorp/consul/acl"
)

func randEnterpriseMeta(_ *acl.EnterpriseMeta, _ fuzz.Continue) {
}
