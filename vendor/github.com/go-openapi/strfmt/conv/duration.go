package conv

import "github.com/go-openapi/strfmt"

// Duration returns a pointer to of the Duration value passed in.
func Duration(v strfmt.Duration) *strfmt.Duration {
	return &v
}

// DurationValue returns the value of the Duration pointer passed in or
// the default value if the pointer is nil.
func DurationValue(v *strfmt.Duration) strfmt.Duration {
	if v == nil {
		return strfmt.Duration(0)
	}

	return *v
}
