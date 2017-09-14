package response

import "fmt"

// Class holds sets of Types
type Class int

const (
	// All is a meta class encompassing all the classes.
	All Class = iota
	// Success is a class for a successful response.
	Success
	// Denial is a class for denying existence (NXDOMAIN, or a nodata: type does not exist)
	Denial
	// Error is a class for errors, right now defined as not Success and not Denial
	Error
)

func (c Class) String() string {
	switch c {
	case All:
		return "all"
	case Success:
		return "success"
	case Denial:
		return "denial"
	case Error:
		return "error"
	}
	return ""
}

// ClassFromString returns the class from the string s. If not class matches
// the All class and an error are returned
func ClassFromString(s string) (Class, error) {
	switch s {
	case "all":
		return All, nil
	case "success":
		return Success, nil
	case "denial":
		return Denial, nil
	case "error":
		return Error, nil
	}
	return All, fmt.Errorf("invalid Class: %s", s)
}

// Classify classifies the Type t, it returns its Class.
func Classify(t Type) Class {
	switch t {
	case NoError, Delegation:
		return Success
	case NameError, NoData:
		return Denial
	case OtherError:
		fallthrough
	default:
		return Error
	}
}
