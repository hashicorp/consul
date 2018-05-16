package msgfmt

type strFormatter int

func (formatter strFormatter) Format(space []byte, kv []interface{}) []byte {
	return append(space, kv[formatter].(string)...)
}
