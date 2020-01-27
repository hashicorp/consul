package lib

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
		in, out  string
		skip     []string
		skipTree []string
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
		{
			// inspired by the 'config_entries.bootstrap.*' structure for configs
			in: `
			{
				"a": [
					{
						"b": [
							{
								"c": "val1",
								"d": {
									"foo": "bar"
								},
								"e": [
									{
										"super": "duper"
									}
								]
							}
						]
					}
				]
			}
			`,
			out: `
			{
				"a": {
					"b": [
						{
							"c": "val1",
							"d": {
								"foo": "bar"
							},
							"e": [
								{
									"super": "duper"
								}
							]
						}
					]
				}
			}
			`,
			skipTree: []string{"a.b"},
		},
	}

	for i, tt := range tests {
		desc := fmt.Sprintf("%02d: %s -> %s skip: %v", i, tt.in, tt.out, tt.skip)
		t.Run(desc, func(t *testing.T) {
			out := PatchSliceOfMaps(parse(tt.in), tt.skip, tt.skipTree)
			if got, want := out, parse(tt.out); !reflect.DeepEqual(got, want) {
				t.Fatalf("\ngot  %#v\nwant %#v", got, want)
			}
		})
	}
}
