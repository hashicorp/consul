package prepared_query

import (
	"fmt"
	"reflect"
)

// visitor is a function that will get called for each string element of a
// structure.
type visitor func(path string, v reflect.Value) error

// visit calls the visitor function for each string it finds, and will descend
// recursively into structures and slices. If any visitor returns an error then
// the search will stop and that error will be returned.
func visit(path string, v reflect.Value, t reflect.Type, fn visitor) error {
	switch v.Kind() {
	case reflect.String:
		return fn(path, v)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			vf := v.Field(i)
			tf := t.Field(i)
			newPath := fmt.Sprintf("%s.%s", path, tf.Name)
			if err := visit(newPath, vf, tf.Type, fn); err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			vi := v.Index(i)
			ti := vi.Type()
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if err := visit(newPath, vi, ti, fn); err != nil {
				return err
			}
		}
	case reflect.Map:
		for i, key := range v.MapKeys() {
			value := v.MapIndex(key)

			newKey := reflect.New(key.Type()).Elem()
			newKey.SetString(key.String())
			newValue := reflect.New(value.Type()).Elem()
			newValue.SetString(value.String())

			if err := visit(fmt.Sprintf("%s.keys[%d]", path, i), newKey, newKey.Type(), fn); err != nil {
				return err
			}
			if err := visit(fmt.Sprintf("%s[%s]", path, key.String()), newValue, newValue.Type(), fn); err != nil {
				return err
			}
			// delete the old entry and add the new one
			v.SetMapIndex(key, reflect.Value{})
			v.SetMapIndex(newKey, newValue)
		}
	}
	return nil
}

// walk finds all the string elements of a given structure (and its sub-
// structures) and calls the visitor function. Each string found will get
// a unique path computed. If any visitor returns an error then the search
// will stop and that error will be returned.
func walk(obj interface{}, fn visitor) error {
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()
	return visit("", v, t, fn)
}
