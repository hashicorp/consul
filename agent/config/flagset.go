package config

import (
	"strconv"
	"strings"
	"time"
)

// boolPtrValue is a flag.Value which stores the value in a *bool if it
// can be parsed with strconv.ParseBool. If the value was not set the
// pointer is nil.
type boolPtrValue struct {
	v **bool
	b bool
}

func newBoolPtrValue(p **bool) *boolPtrValue {
	return &boolPtrValue{p, false}
}

func (s *boolPtrValue) IsBoolFlag() bool { return true }

func (s *boolPtrValue) Set(val string) error {
	b, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}
	*s.v, s.b = &b, true
	return nil
}

func (s *boolPtrValue) Get() interface{} {
	if s.b {
		return *s.v
	}
	return (*bool)(nil)
}

func (s *boolPtrValue) String() string {
	if s.b {
		return strconv.FormatBool(**s.v)
	}
	return ""
}

// durationPtrValue is a flag.Value which stores the value in a
// *time.Duration if it can be parsed with time.ParseDuration. If the
// value was not set the pointer is nil.
type durationPtrValue struct {
	v **time.Duration
	b bool
}

func newDurationPtrValue(p **time.Duration) *durationPtrValue {
	return &durationPtrValue{p, false}
}

func (s *durationPtrValue) Set(val string) error {
	d, err := time.ParseDuration(val)
	if err != nil {
		return err
	}
	*s.v, s.b = &d, true
	return nil
}

func (s *durationPtrValue) Get() interface{} {
	if s.b {
		return *s.v
	}
	return (*time.Duration)(nil)
}

func (s *durationPtrValue) String() string {
	if s.b {
		return (*(*s).v).String()
	}
	return ""
}

// intPtrValue is a flag.Value which stores the value in a *int if it
// can be parsed with strconv.Atoi. If the value was not set the pointer
// is nil.
type intPtrValue struct {
	v **int
	b bool
}

func newIntPtrValue(p **int) *intPtrValue {
	return &intPtrValue{p, false}
}

func (s *intPtrValue) Set(val string) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return err
	}
	*s.v, s.b = &n, true
	return nil
}

func (s *intPtrValue) Get() interface{} {
	if s.b {
		return *s.v
	}
	return (*int)(nil)
}

func (s *intPtrValue) String() string {
	if s.b {
		return strconv.Itoa(**s.v)
	}
	return ""
}

// stringMapValue is a flag.Value which stores the value in a map[string]string if the
// value is in "key:value" format. This can be specified multiple times.
type stringMapValue map[string]string

func newStringMapValue(p *map[string]string) *stringMapValue {
	*p = map[string]string{}
	return (*stringMapValue)(p)
}

func (s *stringMapValue) Set(val string) error {
	p := strings.SplitN(val, ":", 2)
	k, v := p[0], ""
	if len(p) == 2 {
		v = p[1]
	}
	(*s)[k] = v
	return nil
}

func (s *stringMapValue) Get() interface{} {
	return s
}

func (s *stringMapValue) String() string {
	var x []string
	for k, v := range *s {
		if v == "" {
			x = append(x, k)
		} else {
			x = append(x, k+":"+v)
		}
	}
	return strings.Join(x, " ")
}

// stringPtrValue is a flag.Value which stores the value in a *string.
// If the value was not set the pointer is nil.
type stringPtrValue struct {
	v **string
	b bool
}

func newStringPtrValue(p **string) *stringPtrValue {
	return &stringPtrValue{p, false}
}

func (s *stringPtrValue) Set(val string) error {
	*s.v, s.b = &val, true
	return nil
}

func (s *stringPtrValue) Get() interface{} {
	if s.b {
		return *s.v
	}
	return (*string)(nil)
}

func (s *stringPtrValue) String() string {
	if s.b {
		return **s.v
	}
	return ""
}

// stringSliceValue is a flag.Value which appends the value to a []string.
// This can be specified multiple times.
type stringSliceValue []string

func newStringSliceValue(p *[]string) *stringSliceValue {
	return (*stringSliceValue)(p)
}

func (s *stringSliceValue) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func (s *stringSliceValue) Get() interface{} {
	return s
}

func (s *stringSliceValue) String() string {
	return strings.Join(*s, " ")
}
