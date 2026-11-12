//go:build go1.18
// +build go1.18

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
)

// FuzzACLTokenUnmarshal tests ACL token JSON parsing with
// arbitrary attacker-controlled JSON input.
//
// ACL tokens control all authorization in Consul. The token
// JSON parser is a pre-auth boundary — a bug here = privilege
// escalation across the entire service mesh.
//
// 30 GitHub Security Advisories exist for HashiCorp Consul.
func FuzzACLTokenUnmarshal(f *testing.F) {
	f.Add([]byte(`{"AccessorID":"test","SecretID":"secret"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))
	f.Add([]byte(`{`))
	f.Add(make([]byte, 10000))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			return
		}
		var token structs.ACLToken
		_ = token.UnmarshalJSON(data)
	})
}

// FuzzACLPolicyUnmarshal tests ACL policy JSON parsing with
// arbitrary JSON input. ACL policies define the rules that
// control access in Consul.
func FuzzACLPolicyUnmarshal(f *testing.F) {
	f.Add([]byte(`{"ID":"test","Name":"test-policy","Rules":"node { policy = \"read\" }"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			return
		}
		var policy structs.ACLPolicy
		_ = policy.UnmarshalJSON(data)
	})
}
