package maps

func SliceOfKeys[K comparable, V any](m map[K]V) []K {
	if len(m) == 0 {
		return nil
	}
	res := make([]K, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	return res
}

func SliceOfValues[K comparable, V any](m map[K]V) []V {
	if len(m) == 0 {
		return nil
	}
	res := make([]V, 0, len(m))
	for _, v := range m {
		res = append(res, v)
	}
	return res
}
