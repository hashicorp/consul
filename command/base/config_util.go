package base

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/mitchellh/mapstructure"
)

// TODO (slackpad) - Trying out a different pattern here for config handling.
// These classes support the flag.Value interface but work in a manner where
// we can tell if they have been set. This lets us work with an all-pointer
// config structure and merge it in a clean-ish way. If this ends up being a
// good pattern we should pull this out into a reusable library.

// configDecodeHook should be passed to mapstructure in order to decode into
// the *Value objects here.
var configDecodeHook = mapstructure.ComposeDecodeHookFunc(
	boolToBoolValueFunc(),
	stringToDurationValueFunc(),
	stringToStringValueFunc(),
	float64ToUintValueFunc(),
)

// boolValue provides a flag value that's aware if it has been set.
type boolValue struct {
	v *bool
}

// See flag.Value.
func (b *boolValue) IsBoolFlag() bool {
	return true
}

// Merge will overlay this value if it has been set.
func (b *boolValue) Merge(onto *bool) {
	if b.v != nil {
		*onto = *(b.v)
	}
}

// See flag.Value.
func (b *boolValue) Set(v string) error {
	if b.v == nil {
		b.v = new(bool)
	}
	var err error
	*(b.v), err = strconv.ParseBool(v)
	return err
}

// See flag.Value.
func (b *boolValue) String() string {
	var current bool
	if b.v != nil {
		current = *(b.v)
	}
	return fmt.Sprintf("%v", current)
}

// boolToBoolValueFunc is a mapstructure hook that looks for an incoming bool
// mapped to a boolValue and does the translation.
func boolToBoolValueFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.Bool {
			return data, nil
		}

		val := boolValue{}
		if t != reflect.TypeOf(val) {
			return data, nil
		}

		val.v = new(bool)
		*(val.v) = data.(bool)
		return val, nil
	}
}

// durationValue provides a flag value that's aware if it has been set.
type durationValue struct {
	v *time.Duration
}

// Merge will overlay this value if it has been set.
func (d *durationValue) Merge(onto *time.Duration) {
	if d.v != nil {
		*onto = *(d.v)
	}
}

// See flag.Value.
func (d *durationValue) Set(v string) error {
	if d.v == nil {
		d.v = new(time.Duration)
	}
	var err error
	*(d.v), err = time.ParseDuration(v)
	return err
}

// See flag.Value.
func (d *durationValue) String() string {
	var current time.Duration
	if d.v != nil {
		current = *(d.v)
	}
	return current.String()
}

// stringToDurationValueFunc is a mapstructure hook that looks for an incoming
// string mapped to a durationValue and does the translation.
func stringToDurationValueFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}

		val := durationValue{}
		if t != reflect.TypeOf(val) {
			return data, nil
		}
		if err := val.Set(data.(string)); err != nil {
			return nil, err
		}
		return val, nil
	}
}

// stringValue provides a flag value that's aware if it has been set.
type stringValue struct {
	v *string
}

// Merge will overlay this value if it has been set.
func (s *stringValue) Merge(onto *string) {
	if s.v != nil {
		*onto = *(s.v)
	}
}

// See flag.Value.
func (s *stringValue) Set(v string) error {
	if s.v == nil {
		s.v = new(string)
	}
	*(s.v) = v
	return nil
}

// See flag.Value.
func (s *stringValue) String() string {
	var current string
	if s.v != nil {
		current = *(s.v)
	}
	return current
}

// stringToStringValueFunc is a mapstructure hook that looks for an incoming
// string mapped to a stringValue and does the translation.
func stringToStringValueFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}

		val := stringValue{}
		if t != reflect.TypeOf(val) {
			return data, nil
		}
		val.v = new(string)
		*(val.v) = data.(string)
		return val, nil
	}
}

// uintValue provides a flag value that's aware if it has been set.
type uintValue struct {
	v *uint
}

// Merge will overlay this value if it has been set.
func (u *uintValue) Merge(onto *uint) {
	if u.v != nil {
		*onto = *(u.v)
	}
}

// See flag.Value.
func (u *uintValue) Set(v string) error {
	if u.v == nil {
		u.v = new(uint)
	}
	parsed, err := strconv.ParseUint(v, 0, 64)
	*(u.v) = (uint)(parsed)
	return err
}

// See flag.Value.
func (u *uintValue) String() string {
	var current uint
	if u.v != nil {
		current = *(u.v)
	}
	return fmt.Sprintf("%v", current)
}

// float64ToUintValueFunc is a mapstructure hook that looks for an incoming
// float64 mapped to a uintValue and does the translation.
func float64ToUintValueFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.Float64 {
			return data, nil
		}

		val := uintValue{}
		if t != reflect.TypeOf(val) {
			return data, nil
		}

		fv := data.(float64)
		if fv < 0 {
			return nil, fmt.Errorf("value cannot be negative")
		}

		// The standard guarantees at least this, and this is fine for
		// values we expect to use in configs vs. being fancy with the
		// machine's size for uint.
		if fv > (1<<32 - 1) {
			return nil, fmt.Errorf("value is too large")
		}

		val.v = new(uint)
		*(val.v) = (uint)(fv)
		return val, nil
	}
}

// visitFn is a callback that gets a chance to visit each file found during a
// traversal with visit().
type visitFn func(path string) error

// visit will call the visitor function on the path if it's a file, or for each
// file in the path if it's a directory. Directories will not be recursed into,
// and files in the directory will be visited in alphabetical order.
func visit(path string, visitor visitFn) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error reading %q: %v", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("error checking %q: %v", path, err)
	}

	if !fi.IsDir() {
		if err := visitor(path); err != nil {
			return fmt.Errorf("error in %q: %v", path, err)
		}
		return nil
	}

	contents, err := f.Readdir(-1)
	if err != nil {
		return fmt.Errorf("error listing %q: %v", path, err)
	}

	sort.Sort(dirEnts(contents))
	for _, fi := range contents {
		if fi.IsDir() {
			continue
		}

		fullPath := filepath.Join(path, fi.Name())
		if err := visitor(fullPath); err != nil {
			return fmt.Errorf("error in %q: %v", fullPath, err)
		}
	}

	return nil
}

// dirEnts applies sort.Interface to directory entries for sorting by name.
type dirEnts []os.FileInfo

// See sort.Interface.
func (d dirEnts) Len() int {
	return len(d)
}

// See sort.Interface.
func (d dirEnts) Less(i, j int) bool {
	return d[i].Name() < d[j].Name()
}

// See sort.Interface.
func (d dirEnts) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
