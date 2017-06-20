package discover

import (
	"errors"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		s   string
		c   map[string]string
		err error
	}{
		{"", nil, nil},
		{"  ", nil, nil},
		{"provider=aws foo", nil, errors.New(`discover: invalid format: foo`)},
		{"provider=aws region=a+a tag_key=b+b tag_value=c+c access_key_id=d+d secret_access_key=e+e",
			map[string]string{
				"provider":          "aws",
				"region":            "a a",
				"tag_key":           "b b",
				"tag_value":         "c c",
				"access_key_id":     "d d",
				"secret_access_key": "e e",
			},
			nil,
		},
		{"provider=azure tag_name=a+a tag_value=b+b client_id=c+c tenant_id=d+d subscription_id=e+e secret_access_key=f+f",
			map[string]string{
				"provider":          "azure",
				"tag_name":          "a a",
				"tag_value":         "b b",
				"client_id":         "c c",
				"tenant_id":         "d d",
				"subscription_id":   "e e",
				"secret_access_key": "f f",
			},
			nil,
		},
		{"provider=gce project_name=a+a zone_pattern=b+b tag_value=c+c credentials_file=d+d",
			map[string]string{
				"provider":         "gce",
				"project_name":     "a a",
				"zone_pattern":     "b b",
				"tag_value":        "c c",
				"credentials_file": "d d",
			},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			c, err := Parse(tt.s)
			if got, want := err, tt.err; !reflect.DeepEqual(got, want) {
				t.Fatalf("got error %v want %v", got, want)
			}
			if got, want := c, tt.c; !reflect.DeepEqual(got, want) {
				t.Fatalf("got config %#v want %#v", got, want)
			}
		})
	}
}
