// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discoverychain

type stringStack []string

func (s stringStack) Len() int {
	return len(s)
}

func (s *stringStack) Push(v string) {
	*s = append(*s, v)
}

func (s *stringStack) Pop() (string, bool) {
	if len(*s) == 0 {
		return "", false
	}

	size := len(*s)

	v := (*s)[size-1]
	*s = (*s)[0 : size-1]
	return v, true
}

func (s stringStack) Peek() (string, bool) {
	if len(s) == 0 {
		return "", false
	}
	return s[len(s)-1], true
}

// Items returns the underlying slice. The first thing Pushed onto the stack is
// in the 0th slice position.
func (s stringStack) Items() []string {
	return s
}
