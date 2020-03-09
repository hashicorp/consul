package lib

// StringSliceEqual compares two string slices for equality. Both the existence
// of the elements and the order of those elements matter for equality. Empty
// slices are treated identically to nil slices.
func StringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
