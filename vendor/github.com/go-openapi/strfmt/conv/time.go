package conv

import "github.com/go-openapi/strfmt"

// DateTime returns a pointer to of the DateTime value passed in.
func DateTime(v strfmt.DateTime) *strfmt.DateTime {
	return &v
}

// DateTimeValue returns the value of the DateTime pointer passed in or
// the default value if the pointer is nil.
func DateTimeValue(v *strfmt.DateTime) strfmt.DateTime {
	if v == nil {
		return strfmt.DateTime{}
	}

	return *v
}
