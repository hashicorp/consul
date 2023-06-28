package testingconsul

func MergeSlices[V any](x, y []V) []V {
	switch {
	case len(x) == 0 && len(y) == 0:
		return nil
	case len(x) == 0:
		return y
	case len(y) == 0:
		return x
	}

	out := make([]V, 0, len(x)+len(y))
	out = append(out, x...)
	out = append(out, y...)
	return out
}
