package msgfmt

type fixedFormatter string

func (formatter fixedFormatter) Format(space []byte, kv []interface{}) []byte {
	return append(space, formatter...)
}