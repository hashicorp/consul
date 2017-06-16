package discover

import (
	"errors"
	"reflect"
	"testing"

	"github.com/hashicorp/discover/aws"
	"github.com/hashicorp/discover/azure"
	"github.com/hashicorp/discover/gce"
)

func TestParse(t *testing.T) {
	tests := []struct {
		s   string
		c   interface{}
		err error
	}{
		{"", nil, nil},
		{"  ", nil, nil},
		{"provider=foo", nil, errors.New(`discover: unknown provider: "foo"`)},
		{"provider=aws foo", nil, errors.New(`discover: invalid format: foo`)},
		{"provider=aws region=a+a tag_key=b+b tag_value=c+c access_key_id=d+d secret_access_key=e+e",
			&aws.Config{
				Region:          "a a",
				TagKey:          "b b",
				TagValue:        "c c",
				AccessKeyID:     "d d",
				SecretAccessKey: "e e",
			},
			nil,
		},
		{"provider=azure tag_name=a+a tag_value=b+b client_id=c+c tenant_id=d+d subscription_id=e+e secret_access_key=f+f",
			&azure.Config{
				TagName:         "a a",
				TagValue:        "b b",
				ClientID:        "c c",
				TenantID:        "d d",
				SubscriptionID:  "e e",
				SecretAccessKey: "f f",
			},
			nil,
		},
		{"provider=gce project_name=a+a zone_pattern=b+b tag_value=c+c credentials_file=d+d",
			&gce.Config{
				ProjectName:     "a a",
				ZonePattern:     "b b",
				TagValue:        "c c",
				CredentialsFile: "d d",
			},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			c, err := parse(tt.s)
			if got, want := err, tt.err; !reflect.DeepEqual(got, want) {
				t.Fatalf("got error %v want %v", got, want)
			}
			if got, want := c, tt.c; !reflect.DeepEqual(got, want) {
				t.Fatalf("got config %#v want %#v", got, want)
			}
		})
	}
}
