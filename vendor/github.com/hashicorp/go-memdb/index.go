package memdb

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
)

// Indexer is an interface used for defining indexes
type Indexer interface {
	// ExactFromArgs is used to build an exact index lookup
	// based on arguments
	FromArgs(args ...interface{}) ([]byte, error)
}

// SingleIndexer is an interface used for defining indexes
// generating a single entry per object
type SingleIndexer interface {
	// FromObject is used to extract an index value from an
	// object or to indicate that the index value is missing.
	FromObject(raw interface{}) (bool, []byte, error)
}

// MultiIndexer is an interface used for defining indexes
// generating multiple entries per object
type MultiIndexer interface {
	// FromObject is used to extract index values from an
	// object or to indicate that the index value is missing.
	FromObject(raw interface{}) (bool, [][]byte, error)
}

// PrefixIndexer can optionally be implemented for any
// indexes that support prefix based iteration. This may
// not apply to all indexes.
type PrefixIndexer interface {
	// PrefixFromArgs returns a prefix that should be used
	// for scanning based on the arguments
	PrefixFromArgs(args ...interface{}) ([]byte, error)
}

// StringFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field.
type StringFieldIndex struct {
	Field     string
	Lowercase bool
}

func (s *StringFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(s.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", s.Field, obj)
	}

	val := fv.String()
	if val == "" {
		return false, nil, nil
	}

	if s.Lowercase {
		val = strings.ToLower(val)
	}

	// Add the null character as a terminator
	val += "\x00"
	return true, []byte(val), nil
}

func (s *StringFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	if s.Lowercase {
		arg = strings.ToLower(arg)
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func (s *StringFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// StringSliceFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field.
type StringSliceFieldIndex struct {
	Field     string
	Lowercase bool
}

func (s *StringSliceFieldIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(s.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", s.Field, obj)
	}

	if fv.Kind() != reflect.Slice || fv.Type().Elem().Kind() != reflect.String {
		return false, nil, fmt.Errorf("field '%s' is not a string slice", s.Field)
	}

	length := fv.Len()
	vals := make([][]byte, 0, length)
	for i := 0; i < fv.Len(); i++ {
		val := fv.Index(i).String()
		if val == "" {
			continue
		}

		if s.Lowercase {
			val = strings.ToLower(val)
		}

		// Add the null character as a terminator
		val += "\x00"
		vals = append(vals, []byte(val))
	}
	if len(vals) == 0 {
		return false, nil, nil
	}
	return true, vals, nil
}

func (s *StringSliceFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	if s.Lowercase {
		arg = strings.ToLower(arg)
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func (s *StringSliceFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// StringMapFieldIndex is used to extract a field of type map[string]string
// from an object using reflection and builds an index on that field.
type StringMapFieldIndex struct {
	Field     string
	Lowercase bool
}

var MapType = reflect.MapOf(reflect.TypeOf(""), reflect.TypeOf("")).Kind()

func (s *StringMapFieldIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(s.Field)
	if !fv.IsValid() {
		return false, nil, fmt.Errorf("field '%s' for %#v is invalid", s.Field, obj)
	}

	if fv.Kind() != MapType {
		return false, nil, fmt.Errorf("field '%s' is not a map[string]string", s.Field)
	}

	length := fv.Len()
	vals := make([][]byte, 0, length)
	for _, key := range fv.MapKeys() {
		k := key.String()
		if k == "" {
			continue
		}
		val := fv.MapIndex(key).String()

		if s.Lowercase {
			k = strings.ToLower(k)
			val = strings.ToLower(val)
		}

		// Add the null character as a terminator
		k += "\x00" + val + "\x00"

		vals = append(vals, []byte(k))
	}
	if len(vals) == 0 {
		return false, nil, nil
	}
	return true, vals, nil
}

func (s *StringMapFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) > 2 || len(args) == 0 {
		return nil, fmt.Errorf("must provide one or two arguments")
	}
	key, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	if s.Lowercase {
		key = strings.ToLower(key)
	}
	// Add the null character as a terminator
	key += "\x00"

	if len(args) == 2 {
		val, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("argument must be a string: %#v", args[1])
		}
		if s.Lowercase {
			val = strings.ToLower(val)
		}
		// Add the null character as a terminator
		key += val + "\x00"
	}

	return []byte(key), nil
}

// UintFieldIndex is used to extract a uint field from an object using
// reflection and builds an index on that field.
type UintFieldIndex struct {
	Field string
}

func (u *UintFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(u.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", u.Field, obj)
	}

	// Check the type
	k := fv.Kind()
	size, ok := IsUintType(k)
	if !ok {
		return false, nil, fmt.Errorf("field %q is of type %v; want a uint", u.Field, k)
	}

	// Get the value and encode it
	val := fv.Uint()
	buf := make([]byte, size)
	binary.PutUvarint(buf, val)

	return true, buf, nil
}

func (u *UintFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}

	v := reflect.ValueOf(args[0])
	if !v.IsValid() {
		return nil, fmt.Errorf("%#v is invalid", args[0])
	}

	k := v.Kind()
	size, ok := IsUintType(k)
	if !ok {
		return nil, fmt.Errorf("arg is of type %v; want a uint", k)
	}

	val := v.Uint()
	buf := make([]byte, size)
	binary.PutUvarint(buf, val)

	return buf, nil
}

// IsUintType returns whether the passed type is a type of uint and the number
// of bytes needed to encode the type.
func IsUintType(k reflect.Kind) (size int, okay bool) {
	switch k {
	case reflect.Uint:
		return binary.MaxVarintLen64, true
	case reflect.Uint8:
		return 2, true
	case reflect.Uint16:
		return binary.MaxVarintLen16, true
	case reflect.Uint32:
		return binary.MaxVarintLen32, true
	case reflect.Uint64:
		return binary.MaxVarintLen64, true
	default:
		return 0, false
	}
}

// UUIDFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field by treating
// it as a UUID. This is an optimization to using a StringFieldIndex
// as the UUID can be more compactly represented in byte form.
type UUIDFieldIndex struct {
	Field string
}

func (u *UUIDFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(u.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", u.Field, obj)
	}

	val := fv.String()
	if val == "" {
		return false, nil, nil
	}

	buf, err := u.parseString(val, true)
	return true, buf, err
}

func (u *UUIDFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	switch arg := args[0].(type) {
	case string:
		return u.parseString(arg, true)
	case []byte:
		if len(arg) != 16 {
			return nil, fmt.Errorf("byte slice must be 16 characters")
		}
		return arg, nil
	default:
		return nil,
			fmt.Errorf("argument must be a string or byte slice: %#v", args[0])
	}
}

func (u *UUIDFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	switch arg := args[0].(type) {
	case string:
		return u.parseString(arg, false)
	case []byte:
		return arg, nil
	default:
		return nil,
			fmt.Errorf("argument must be a string or byte slice: %#v", args[0])
	}
}

// parseString parses a UUID from the string. If enforceLength is false, it will
// parse a partial UUID. An error is returned if the input, stripped of hyphens,
// is not even length.
func (u *UUIDFieldIndex) parseString(s string, enforceLength bool) ([]byte, error) {
	// Verify the length
	l := len(s)
	if enforceLength && l != 36 {
		return nil, fmt.Errorf("UUID must be 36 characters")
	} else if l > 36 {
		return nil, fmt.Errorf("Invalid UUID length. UUID have 36 characters; got %d", l)
	}

	hyphens := strings.Count(s, "-")
	if hyphens > 4 {
		return nil, fmt.Errorf(`UUID should have maximum of 4 "-"; got %d`, hyphens)
	}

	// The sanitized length is the length of the original string without the "-".
	sanitized := strings.Replace(s, "-", "", -1)
	sanitizedLength := len(sanitized)
	if sanitizedLength%2 != 0 {
		return nil, fmt.Errorf("Input (without hyphens) must be even length")
	}

	dec, err := hex.DecodeString(sanitized)
	if err != nil {
		return nil, fmt.Errorf("Invalid UUID: %v", err)
	}

	return dec, nil
}

// FieldSetIndex is used to extract a field from an object using reflection and
// builds an index on whether the field is set by comparing it against its
// type's nil value.
type FieldSetIndex struct {
	Field string
}

func (f *FieldSetIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(f.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", f.Field, obj)
	}

	if fv.Interface() == reflect.Zero(fv.Type()).Interface() {
		return true, []byte{0}, nil
	}

	return true, []byte{1}, nil
}

func (f *FieldSetIndex) FromArgs(args ...interface{}) ([]byte, error) {
	return fromBoolArgs(args)
}

// ConditionalIndex builds an index based on a condition specified by a passed
// user function. This function may examine the passed object and return a
// boolean to encapsulate an arbitrarily complex conditional.
type ConditionalIndex struct {
	Conditional ConditionalIndexFunc
}

// ConditionalIndexFunc is the required function interface for a
// ConditionalIndex.
type ConditionalIndexFunc func(obj interface{}) (bool, error)

func (c *ConditionalIndex) FromObject(obj interface{}) (bool, []byte, error) {
	// Call the user's function
	res, err := c.Conditional(obj)
	if err != nil {
		return false, nil, fmt.Errorf("ConditionalIndexFunc(%#v) failed: %v", obj, err)
	}

	if res {
		return true, []byte{1}, nil
	}

	return true, []byte{0}, nil
}

func (c *ConditionalIndex) FromArgs(args ...interface{}) ([]byte, error) {
	return fromBoolArgs(args)
}

// fromBoolArgs is a helper that expects only a single boolean argument and
// returns a single length byte array containing either a one or zero depending
// on whether the passed input is true or false respectively.
func fromBoolArgs(args []interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}

	if val, ok := args[0].(bool); !ok {
		return nil, fmt.Errorf("argument must be a boolean type: %#v", args[0])
	} else if val {
		return []byte{1}, nil
	}

	return []byte{0}, nil
}

// CompoundIndex is used to build an index using multiple sub-indexes
// Prefix based iteration is supported as long as the appropriate prefix
// of indexers support it. All sub-indexers are only assumed to expect
// a single argument.
type CompoundIndex struct {
	Indexes []Indexer

	// AllowMissing results in an index based on only the indexers
	// that return data. If true, you may end up with 2/3 columns
	// indexed which might be useful for an index scan. Otherwise,
	// the CompoundIndex requires all indexers to be satisfied.
	AllowMissing bool
}

func (c *CompoundIndex) FromObject(raw interface{}) (bool, []byte, error) {
	var out []byte
	for i, idxRaw := range c.Indexes {
		idx, ok := idxRaw.(SingleIndexer)
		if !ok {
			return false, nil, fmt.Errorf("sub-index %d error: %s", i, "sub-index must be a SingleIndexer")
		}
		ok, val, err := idx.FromObject(raw)
		if err != nil {
			return false, nil, fmt.Errorf("sub-index %d error: %v", i, err)
		}
		if !ok {
			if c.AllowMissing {
				break
			} else {
				return false, nil, nil
			}
		}
		out = append(out, val...)
	}
	return true, out, nil
}

func (c *CompoundIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != len(c.Indexes) {
		return nil, fmt.Errorf("less arguments than index fields")
	}
	var out []byte
	for i, arg := range args {
		val, err := c.Indexes[i].FromArgs(arg)
		if err != nil {
			return nil, fmt.Errorf("sub-index %d error: %v", i, err)
		}
		out = append(out, val...)
	}
	return out, nil
}

func (c *CompoundIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	if len(args) > len(c.Indexes) {
		return nil, fmt.Errorf("more arguments than index fields")
	}
	var out []byte
	for i, arg := range args {
		if i+1 < len(args) {
			val, err := c.Indexes[i].FromArgs(arg)
			if err != nil {
				return nil, fmt.Errorf("sub-index %d error: %v", i, err)
			}
			out = append(out, val...)
		} else {
			prefixIndexer, ok := c.Indexes[i].(PrefixIndexer)
			if !ok {
				return nil, fmt.Errorf("sub-index %d does not support prefix scanning", i)
			}
			val, err := prefixIndexer.PrefixFromArgs(arg)
			if err != nil {
				return nil, fmt.Errorf("sub-index %d error: %v", i, err)
			}
			out = append(out, val...)
		}
	}
	return out, nil
}
