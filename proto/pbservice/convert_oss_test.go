//go:build !consulent
// +build !consulent

package pbservice

import (
	fuzz "github.com/google/gofuzz"

	"github.com/hashicorp/consul/agent/structs"
)

func randEnterpriseMeta(_ *structs.EnterpriseMeta, _ fuzz.Continue) {
}
