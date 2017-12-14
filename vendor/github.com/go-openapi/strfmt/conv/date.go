package conv

import "github.com/go-openapi/strfmt"

// Date returns a pointer to of the Date value passed in.
func Date(v strfmt.Date) *strfmt.Date {
	return &v
}

// DateValue returns the value of the Date pointer passed in or
// the default value if the pointer is nil.
func DateValue(v *strfmt.Date) strfmt.Date {
	if v == nil {
		return strfmt.Date{}
	}

	return *v
}
