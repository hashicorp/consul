package test

func init() {
	nilSlice := []string(nil)
	marshalCases = append(marshalCases,
		[]interface{}{"hello"},
		nilSlice,
		&nilSlice,
	)
	unmarshalCases = append(unmarshalCases, unmarshalCase{
		ptr:   (*[]string)(nil),
		input: "null",
	}, unmarshalCase{
		ptr:   (*[]string)(nil),
		input: "[]",
	})
}
