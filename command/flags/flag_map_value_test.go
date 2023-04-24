package flags

import (
	"fmt"
	"testing"
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
