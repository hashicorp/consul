// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTxnOp_FieldCount_CoversAllFields verifies FieldCount counts every
// operation type on TxnOp.
func TestTxnOp_FieldCount_CoversAllFields(t *testing.T) {
	typ := reflect.TypeOf(TxnOp{})

	all := reflect.New(typ).Elem()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		require.Equal(t, reflect.Ptr, f.Type.Kind(),
			"TxnOp.%s is not a pointer; update FieldCount and this test", f.Name)

		var op TxnOp
		v := reflect.ValueOf(&op).Elem().Field(i)
		v.Set(reflect.New(f.Type.Elem()))
		require.Equal(t, 1, op.FieldCount(),
			"TxnOp.%s is not counted by FieldCount; add it", f.Name)

		all.Field(i).Set(reflect.New(f.Type.Elem()))
	}

	full := all.Interface().(TxnOp)
	require.Equal(t, typ.NumField(), full.FieldCount())
}
