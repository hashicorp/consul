package flags

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagMapValueSet(t *testing.T) {
	t.Parallel()

	t.Run("missing =", func(t *testing.T) {

		f := new(FlagMapValue)
		if err := f.Set("foo"); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("sets", func(t *testing.T) {

		f := new(FlagMapValue)
		if err := f.Set("foo=bar"); err != nil {
			t.Fatal(err)
		}

		r, ok := (*f)["foo"]
		if !ok {
			t.Errorf("missing value: %#v", f)
		}
		if exp := "bar"; r != exp {
			t.Errorf("expected %q to be %q", r, exp)
		}
	})

	t.Run("sets multiple", func(t *testing.T) {

		f := new(FlagMapValue)

		r := map[string]string{
			"foo": "bar",
			"zip": "zap",
			"cat": "dog",
		}

		for k, v := range r {
			if err := f.Set(fmt.Sprintf("%s=%s", k, v)); err != nil {
				t.Fatal(err)
			}
		}

		for k, v := range r {
			r, ok := (*f)[k]
			if !ok {
				t.Errorf("missing value %q: %#v", k, f)
			}
			if exp := v; r != exp {
				t.Errorf("expected %q to be %q", r, exp)
			}
		}
	})

	t.Run("overwrites", func(t *testing.T) {

		f := new(FlagMapValue)
		if err := f.Set("foo=bar"); err != nil {
			t.Fatal(err)
		}
		if err := f.Set("foo=zip"); err != nil {
			t.Fatal(err)
		}

		r, ok := (*f)["foo"]
		if !ok {
			t.Errorf("missing value: %#v", f)
		}
		if exp := "zip"; r != exp {
			t.Errorf("expected %q to be %q", r, exp)
		}
	})
}

func TestFlagMapValueMerge(t *testing.T) {
	cases := map[string]struct {
		src FlagMapValue
		dst map[string]string
		exp map[string]string
	}{
		"empty source and destination": {},
		"empty source": {
			dst: map[string]string{
				"key": "val",
			},
			exp: map[string]string{
				"key": "val",
			},
		},
		"empty destination": {
			src: map[string]string{
				"key": "val",
			},
			dst: make(map[string]string),
			exp: map[string]string{
				"key": "val",
			},
		},
		"non-overlapping keys": {
			src: map[string]string{
				"key1": "val1",
			},
			dst: map[string]string{
				"key2": "val2",
			},
			exp: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
		},
		"overlapping keys": {
			src: map[string]string{
				"key1": "val1",
			},
			dst: map[string]string{
				"key1": "val2",
			},
			exp: map[string]string{
				"key1": "val2",
			},
		},
		"multiple keys": {
			src: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			dst: map[string]string{
				"key1": "val2",
				"key3": "val3",
			},
			exp: map[string]string{
				"key1": "val2",
				"key2": "val2",
				"key3": "val3",
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c.src.Merge(c.dst)
			require.Equal(t, c.exp, c.dst)
		})
	}
}
