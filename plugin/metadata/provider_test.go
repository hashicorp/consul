package metadata

import (
	"context"
	"reflect"
	"testing"
)

func TestMD(t *testing.T) {
	tests := []struct {
		addValues      map[string]interface{}
		expectedValues map[string]interface{}
	}{
		{
			// Add initial metadata key/vals
			map[string]interface{}{"key1": "val1", "key2": 2},
			map[string]interface{}{"key1": "val1", "key2": 2},
		},
		{
			// Add additional key/vals.
			map[string]interface{}{"key3": 3, "key4": 4.5},
			map[string]interface{}{"key1": "val1", "key2": 2, "key3": 3, "key4": 4.5},
		},
	}

	// Using one same md and ctx for all test cases
	ctx := context.TODO()
	ctx = context.WithValue(ctx, metadataKey{}, M{})
	m, _ := FromContext(ctx)

	for i, tc := range tests {
		for k, v := range tc.addValues {
			m.SetValue(k, v)
		}
		if !reflect.DeepEqual(tc.expectedValues, map[string]interface{}(m)) {
			t.Errorf("Test %d: Expected %v but got %v", i, tc.expectedValues, m)
		}

		// Make sure that md is recieved from context successfullly
		mFromContext, ok := FromContext(ctx)
		if !ok {
			t.Errorf("Test %d: md is not recieved from the context", i)
		}
		if !reflect.DeepEqual(m, mFromContext) {
			t.Errorf("Test %d: md recieved from context differs from initial. Initial: %v, from context: %v", i, m, mFromContext)
		}
	}
}
