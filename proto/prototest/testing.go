package prototest

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
)

func AssertDeepEqual(t *testing.T, x, y interface{}, opts ...cmp.Option) {
	t.Helper()

	opts = append(opts, cmp.Comparer(proto.Equal))

	if diff := cmp.Diff(x, y, opts...); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}
