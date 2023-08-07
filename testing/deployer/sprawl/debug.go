// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sprawl

import "encoding/json"

func jd(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
