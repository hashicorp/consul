// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	"github.com/stretchr/testify/require"
)

// TestingOldPre1dot7MsgpackHandle is the common configuration pre-1.7.0
var TestingOldPre1dot7MsgpackHandle = &codec.MsgpackHandle{}

// TestMsgpackEncodeDecode is a test helper to easily write a test to verify
// msgpack encoding and decoding using two handles is identical.
func TestMsgpackEncodeDecode(t *testing.T, in interface{}, requireEncoderEquality bool) {
	t.Helper()
	var (
		// This is the common configuration pre-1.7.0
		handle1 = TestingOldPre1dot7MsgpackHandle
		// This is the common configuration post-1.7.0
		handle2 = MsgpackHandle
	)

	// Verify the 3 interface{} args are all pointers to the same kind of type.
	inType := reflect.TypeOf(in)
	require.Equal(t, reflect.Ptr, inType.Kind())

	// Encode using both handles.
	var b1 []byte
	{
		var buf bytes.Buffer
		enc := codec.NewEncoder(&buf, handle1)
		require.NoError(t, enc.Encode(in))
		b1 = buf.Bytes()
	}
	var b2 []byte
	{
		var buf bytes.Buffer
		enc := codec.NewEncoder(&buf, handle2)
		require.NoError(t, enc.Encode(in))
		b2 = buf.Bytes()
	}

	if requireEncoderEquality {
		// The resulting bytes should be identical.
		require.Equal(t, b1, b2)
	}

	// Decode both outputs using both handles.
	t.Run("old encoder and old decoder", func(t *testing.T) {
		out1 := reflect.New(inType.Elem()).Interface()
		dec := codec.NewDecoderBytes(b1, handle1)
		require.NoError(t, dec.Decode(out1))
		require.Equal(t, in, out1)
	})
	t.Run("old encoder and new decoder", func(t *testing.T) {
		out1 := reflect.New(inType.Elem()).Interface()
		dec := codec.NewDecoderBytes(b1, handle2)
		require.NoError(t, dec.Decode(out1))
		require.Equal(t, in, out1)
	})
	t.Run("new encoder and old decoder", func(t *testing.T) {
		out2 := reflect.New(inType.Elem()).Interface()
		dec := codec.NewDecoderBytes(b2, handle1)
		require.NoError(t, dec.Decode(out2))
		require.Equal(t, in, out2)
	})
	t.Run("new encoder and new decoder", func(t *testing.T) {
		out2 := reflect.New(inType.Elem()).Interface()
		dec := codec.NewDecoderBytes(b2, handle2)
		require.NoError(t, dec.Decode(out2))
		require.Equal(t, in, out2)
	})
}
