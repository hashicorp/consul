package utils

import (
	"encoding/json"
	"fmt"

	"github.com/itchyny/gojq"
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

// JQFilter uses the provided "jq" filter to parse json.
// Matching results are returned as a slice of strings.
func JQFilter(config, filter string) ([]string, error) {
	result := []string{}
	query, err := gojq.Parse(filter)
	if err != nil {
		return nil, err
	}

	var m interface{}
	err = json.Unmarshal([]byte(config), &m)
	if err != nil {
		return nil, err
	}

	iter := query.Run(m)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}
		s := fmt.Sprintf("%v", v)
		result = append(result, s)
	}
	return result, nil
}

func IntToPointer(i int) *int {
	return &i
}

func BoolToPointer(b bool) *bool {
	return &b
}

func StringToPointer(s string) *string {
	return &s
}
