package utils

import (
	"github.com/teris-io/shortid"
)

func RandName(name string) string {
	shortID, err := shortid.New(1, shortid.DefaultABC, 6666)
	id, err := shortID.Generate()
	if err != nil {
		return ""
	}
	return name + "-" + id
}

func StringPointer(s string) *string {
	return &s
}
