package sourcepkg

const ExportedConstant = "constant"

var ExportedVar = "var"

type unexportedStruct struct {
	ExportedField   bool
	unexportedField bool
}

func (unexportedStruct) ExportedMethod() {}

func ExportedFunction() *Sample {
	return nil
}

// Sample source struct with mog annotations, used for testing.
//
// mog annotation:
//
// name=Core
// target=github.com/hashicorp/mog/internal/targetpkgone.TheSample
// ignore-fields=ExtraField
type Sample struct {
	unexportedField bool

	BoolField bool
	// mog: target=StringPtrField
	StringField string
	IntField    int
	MapField    map[string]string
}

// ExportedMethod does nothing
func (Sample) ExportedMethod() {}

// godoc on the GenDecl for testing.
type (
	// GroupedSample is a source struct.
	//
	// mog annotation:
	//
	GroupedSample struct {
		StructField Sample
	}

	GroupedNotASourceStruct struct{}
)

// NotASourceStruct is a struct with a comment, but is not a source struct for
// mog.
type NotASourceStruct struct{}

type NotASourceStructNoComment struct{}
