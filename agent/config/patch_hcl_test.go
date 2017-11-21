package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func parse(s string) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		panic(s + ":" + err.Error())
	}
	return m
}

func TestPatchSliceOfMaps(t *testing.T) {
	tests := []struct {
		in, out string
		skip    []string
	}{
		{
			in:  `{"a":{"b":"c"}}`,
			out: `{"a":{"b":"c"}}`,
		},
		{
			in:  `{"a":[{"b":"c"}]}`,
			out: `{"a":{"b":"c"}}`,
		},
		{
			in:  `{"a":[{"b":[{"c":"d"}]}]}`,
			out: `{"a":{"b":{"c":"d"}}}`,
		},
		{
			in:   `{"a":[{"b":"c"}]}`,
			out:  `{"a":[{"b":"c"}]}`,
			skip: []string{"a"},
		},
		{
			in: `{
				"services": [
					{
						"checks": [
							{
								"header": [
									{"a":"b"}
								]
							}
						]
					}
				]
			}`,
			out: `{
				"services": [
					{
						"checks": [
							{
								"header": {"a":"b"}
							}
						]
					}
				]
			}`,
			skip: []string{"services", "services.checks"},
		},
	}

	for i, tt := range tests {
		desc := fmt.Sprintf("%02d: %s -> %s skip: %v", i, tt.in, tt.out, tt.skip)
		t.Run(desc, func(t *testing.T) {
			out := patchSliceOfMaps(parse(tt.in), tt.skip)
			if got, want := out, parse(tt.out); !reflect.DeepEqual(got, want) {
				t.Fatalf("\ngot  %#v\nwant %#v", got, want)
			}
		})
	}
}
