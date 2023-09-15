// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import "encoding/json"

func jd(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
