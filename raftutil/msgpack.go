// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/consul/agent/structs"
	"time"
	"unicode"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
)

// fixTime converts any suspected time.Time binary string representation to time.Time
func fixTime(v interface{}) {
	switch v2 := v.(type) {
	case map[string]interface{}:
		for ek, ev := range v2 {
			if s, ok := ev.(string); ok {
				t, err := maybeDecodeTime(s)
				if err == nil && isReasonableTime(t) {
					v2[ek] = *t
				}
			} else {
				fixTime(ev)
			}
		}
	case []interface{}:
		for _, e := range v2 {
			fixTime(e)
		}
	default:
		return
	}
}

// maybeDecodeTime returns a time.Time representation if the string represents a msgpack
// representation of a date.
func maybeDecodeTime(v string) (*time.Time, error) {
	if isASCII(v) {
		return nil, fmt.Errorf("simple ascii string")
	}

	tt := &time.Time{}
	var err error

	err = tt.UnmarshalBinary([]byte(v))
	if err == nil {
		return tt, nil
	}

	switch len(v) {
	case 4, 8, 12:
	default:
		return nil, fmt.Errorf("bad length: %d", len(v))
	}

	var nb bytes.Buffer
	err = codec.NewEncoder(&nb, structs.MsgpackHandle).Encode(v)
	if err != nil {
		return nil, err
	}

	err = codec.NewDecoder(&nb, structs.MsgpackHandle).Decode(tt)
	if err != nil {
		return nil, err
	}

	return tt, nil
}

// isASCII returns true if all string characters are ASCII characters
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}

	return true
}

// isReasonableTime returns true if the time is within some N years of current time
//
// It's can be used to rule out bad date interpretation (e.g. dates million years away).
func isReasonableTime(t *time.Time) bool {
	if t.IsZero() {
		return true
	}

	now := time.Now()
	return t.Before(now.AddDate(20, 0, 0)) && t.After(now.AddDate(-20, 0, 0))
}
