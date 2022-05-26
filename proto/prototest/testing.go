package prototest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func AssertDeepEqual(t *testing.T, x, y interface{}, opts ...cmp.Option) {
	t.Helper()

	opts = append(opts, protocmp.Transform())

	if diff := cmp.Diff(x, y, opts...); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}
