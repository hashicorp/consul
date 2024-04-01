// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import "encoding/json"

// Dump pretty prints the provided arg as json.
func Dump(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "<ERR: " + err.Error() + ">"
	}
	return string(b)
}
