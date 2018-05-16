package msgfmt

import (
	"encoding/json"
	"fmt"
)

type invalidFormatter string

func (formatter invalidFormatter) Format(space []byte, kv []interface{}) []byte {
	output, err := json.Marshal(kv)
	if err != nil {
		space = append(space, "%! (INVALID FORMAT) "...)
		space = append(space, formatter...)
		space = append(space, "%! (FORMAT ARGS) "...)
		space = append(space, output...)
	} else {
		space = append(space, "%! (INVALID FORMAT) "...)
		space = append(space, formatter...)
		space = append(space, "%! (FORMAT ARGS) "...)
		space = append(space, fmt.Sprintf("%v", kv)...)
	}
	return space
}