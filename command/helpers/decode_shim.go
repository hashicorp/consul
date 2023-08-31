// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package helpers

import (
	"encoding/json"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/hcl"
)

// hclDecode is a modified version of hcl.Decode just for the super general
// purposes here. There's some strange bug in how hcl.Decode decodes json where
//
// { "sub" : { "v1" : { "field" : "value1" }, "v2" : { "field" : "value2" } } }
//
// hcl.Decode-s into:
//
//	map[string]interface {}{
//		"sub":[]map[string]interface {}{
//			map[string]interface {}{
//				"v1":[]map[string]interface {}{
//					map[string]interface {}{
//						"field":"value1"
//					}
//				}
//			},
//			map[string]interface {}{
//				"v2":[]map[string]interface {}{
//					map[string]interface {}{
//						"field":"value2"
//					}
//				}
//			}
//		}
//	}
//
// but json.Unmarshal-s into the more expected:
//
//	map[string]interface {}{
//		"sub":map[string]interface {}{
//			"v1":map[string]interface {}{
//				"field":"value1"
//			},
//			"v2":map[string]interface {}{
//				"field":"value2"
//			}
//		}
//	}
//
// The strange part is that the following HCL:
//
// sub { "v1" = { field = "value1" }, "v2" = { field = "value2" } }
//
// hcl.Decode-s into:
//
//	map[string]interface {}{
//		"sub":[]map[string]interface {}{
//			map[string]interface {}{
//				"v1":[]map[string]interface {}{
//					map[string]interface {}{
//						"field":"value1"
//					}
//				},
//				"v2":[]map[string]interface {}{
//					map[string]interface {}{
//						"field":"value2"
//					}
//				}
//			}
//		}
//	}
//
// Which is the "correct" value assuming you did the patch-slice-of-maps correction.
//
// Given that HCLv1 is basically frozen and the HCL part of it is fine instead
// of trying to track down a weird bug we'll bypass the weird JSON decoder and just use
// the stdlib one.
func hclDecode(out interface{}, in string) error {
	data := []byte(in)
	if isHCL(data) {
		return hcl.Decode(out, in)
	}

	return json.Unmarshal(data, out)
}

// this is an inlined variant of hcl.lexMode()
func isHCL(v []byte) bool {
	var (
		r      rune
		w      int
		offset int
	)

	for {
		r, w = utf8.DecodeRune(v[offset:])
		offset += w
		if unicode.IsSpace(r) {
			continue
		}
		if r == '{' {
			return false
		}
		break
	}

	return true
}
