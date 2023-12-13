// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"bytes"
)

// indexSeparator delimits the segments of our radix tree keys.
const indexSeparator = "\x00"

type Builder bytes.Buffer

func (i *Builder) Raw(v []byte) {
	(*bytes.Buffer)(i).Write(v)
}

func (i *Builder) String(s string) {
	(*bytes.Buffer)(i).WriteString(s)
	(*bytes.Buffer)(i).WriteString(indexSeparator)
}

func (i *Builder) Bytes() []byte {
	return (*bytes.Buffer)(i).Bytes()
}

func (i *Builder) Write(b []byte) (int, error) {
	(*bytes.Buffer)(i).Write(b)
	(*bytes.Buffer)(i).WriteString(indexSeparator)

	return len(b), nil
}
