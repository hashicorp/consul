// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/dns"
)

const MaxNameLength = 63

// ValidateName returns an error a name is not a valid resource name.
// The error will contain reference to what constitutes a valid resource name.
func ValidateName(name string) error {
	if !dns.IsValidLabel(name) || strings.ToLower(name) != name || len(name) > MaxNameLength {
		return fmt.Errorf("a resource name must consist of lower case alphanumeric characters or '-', must start and end with an alphanumeric character and be less than %d characters, got: %q", MaxNameLength+1, name)
	}
	return nil
}
