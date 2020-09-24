package targetpkgone

type TheSample struct {
	BoolField       bool
	StringPtrField  *string
	IntField        int
	ExtraField      string
	unexportedField bool
}