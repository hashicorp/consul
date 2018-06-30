// Go MySQL Driver - A MySQL-Driver for Go's database/sql package
//
// Copyright 2014 The Go-MySQL-Driver Authors. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package mysql

const defaultCollation = "utf8_general_ci"

// A blacklist of collations which is unsafe to interpolate parameters.
// These multibyte encodings may contains 0x5c (`\`) in their trailing bytes.
var unsafeCollations = map[string]bool{
	"big5_chinese_ci":   true,
	"sjis_japanese_ci":  true,
	"gbk_chinese_ci":    true,
	"big5_bin":          true,
	"gb2312_bin":        true,
	"gbk_bin":           true,
	"sjis_bin":          true,
	"cp932_japanese_ci": true,
	"cp932_bin":         true,
}
