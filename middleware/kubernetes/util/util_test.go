package util

import (
	"testing"
)

type InSliceData struct {
	Slice   []string
	String  string
	InSlice bool
}

// Test data for TestStringInSlice cases.
var testdataInSlice = []struct {
	Slice          []string
	String         string
	ExpectedResult bool
}{
	{[]string{"a", "b", "c"}, "a", true},
	{[]string{"a", "b", "c"}, "d", false},
	{[]string{"a", "b", "c"}, "", false},
	{[]string{}, "a", false},
	{[]string{}, "", false},
}

func TestStringInSlice(t *testing.T) {
	for _, example := range testdataInSlice {
		actualResult := StringInSlice(example.String, example.Slice)
		if actualResult != example.ExpectedResult {
			t.Errorf("Expected stringInSlice result '%v' for example string='%v', slice='%v'. Instead got result '%v'.", example.ExpectedResult, example.String, example.Slice, actualResult)
		}
	}
}
