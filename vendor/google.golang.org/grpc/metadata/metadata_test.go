/*
 *
 * Copyright 2014 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package metadata

import (
	"reflect"
	"testing"

	"golang.org/x/net/context"
)

func TestPairsMD(t *testing.T) {
	for _, test := range []struct {
		// input
		kv []string
		// output
		md MD
	}{
		{[]string{}, MD{}},
		{[]string{"k1", "v1", "k1", "v2"}, MD{"k1": []string{"v1", "v2"}}},
	} {
		md := Pairs(test.kv...)
		if !reflect.DeepEqual(md, test.md) {
			t.Fatalf("Pairs(%v) = %v, want %v", test.kv, md, test.md)
		}
	}
}

func TestCopy(t *testing.T) {
	const key, val = "key", "val"
	orig := Pairs(key, val)
	copy := orig.Copy()
	if !reflect.DeepEqual(orig, copy) {
		t.Errorf("copied value not equal to the original, got %v, want %v", copy, orig)
	}
	orig[key][0] = "foo"
	if v := copy[key][0]; v != val {
		t.Errorf("change in original should not affect copy, got %q, want %q", v, val)
	}
}

func TestJoin(t *testing.T) {
	for _, test := range []struct {
		mds  []MD
		want MD
	}{
		{[]MD{}, MD{}},
		{[]MD{Pairs("foo", "bar")}, Pairs("foo", "bar")},
		{[]MD{Pairs("foo", "bar"), Pairs("foo", "baz")}, Pairs("foo", "bar", "foo", "baz")},
		{[]MD{Pairs("foo", "bar"), Pairs("foo", "baz"), Pairs("zip", "zap")}, Pairs("foo", "bar", "foo", "baz", "zip", "zap")},
	} {
		md := Join(test.mds...)
		if !reflect.DeepEqual(md, test.want) {
			t.Errorf("context's metadata is %v, want %v", md, test.want)
		}
	}
}

func TestAppendToOutgoingContext(t *testing.T) {
	// Pre-existing metadata
	ctx := NewOutgoingContext(context.Background(), Pairs("k1", "v1", "k2", "v2"))
	ctx = AppendToOutgoingContext(ctx, "k1", "v3")
	ctx = AppendToOutgoingContext(ctx, "k1", "v4")
	md, ok := FromOutgoingContext(ctx)
	if !ok {
		t.Errorf("Expected MD to exist in ctx, but got none")
	}
	want := Pairs("k1", "v1", "k1", "v3", "k1", "v4", "k2", "v2")
	if !reflect.DeepEqual(md, want) {
		t.Errorf("context's metadata is %v, want %v", md, want)
	}

	// No existing metadata
	ctx = AppendToOutgoingContext(context.Background(), "k1", "v1")
	md, ok = FromOutgoingContext(ctx)
	if !ok {
		t.Errorf("Expected MD to exist in ctx, but got none")
	}
	want = Pairs("k1", "v1")
	if !reflect.DeepEqual(md, want) {
		t.Errorf("context's metadata is %v, want %v", md, want)
	}
}

// Old/slow approach to adding metadata to context
func Benchmark_AddingMetadata_ContextManipulationApproach(b *testing.B) {
	for n := 0; n < b.N; n++ {
		ctx := context.Background()
		md, _ := FromOutgoingContext(ctx)
		NewOutgoingContext(ctx, Join(Pairs("k1", "v1", "k2", "v2"), md))
	}
}

// Newer/faster approach to adding metadata to context
func BenchmarkAppendToOutgoingContext(b *testing.B) {
	for n := 0; n < b.N; n++ {
		AppendToOutgoingContext(context.Background(), "k1", "v1", "k2", "v2")
	}
}

func BenchmarkFromOutgoingContext(b *testing.B) {
	ctx := context.Background()
	ctx = NewOutgoingContext(ctx, MD{"k3": {"v3", "v4"}})
	ctx = AppendToOutgoingContext(ctx, "k1", "v1", "k2", "v2")

	for n := 0; n < b.N; n++ {
		FromOutgoingContext(ctx)
	}
}
