package utils

import (
	"github.com/hashicorp/go-uuid"
)

func RandName(name string) string {
	generateUUID, err := uuid.GenerateUUID()
	if err != nil {
		return ""
	}
	return name + "-" + generateUUID
}

func StringPointer(s string) *string {
	return &s
}
