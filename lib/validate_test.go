package lib

import (
	"regexp"
	"testing"
)

func TestIsValidBasicName(t *testing.T) {
	type testArgs struct {
		value      string
		allowEmpty bool
	}
	tests := []struct {
		name string
		args testArgs
		want bool
	}{
		{
			name: "EmptyName",
			args: testArgs{
				value:      "",
				allowEmpty: true,
			},
			want: true,
		},
		{
			name: "ValidName",
			args: testArgs{
				value:      "custom-datacenter",
				allowEmpty: true,
			},
			want: true,
		},
		{
			name: "InValidName",
			args: testArgs{
				value:      ",",
				allowEmpty: true,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidBasicName(tt.args.value, tt.args.allowEmpty); got != tt.want {
				t.Errorf("IsValidBasicName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateString(t *testing.T) {
	type testArgs struct {
		validator  regexp.Regexp
		value      string
		allowEmpty bool
	}
	tests := []struct {
		name    string
		args    testArgs
		wantErr bool
	}{
		{
			name: "ValidBasicName",
			args: testArgs{
				validator:  *regexp.MustCompile("^[a-z0-9_-]+$"),
				value:      "custom-datacenter",
				allowEmpty: true,
			},
			wantErr: false,
		},
		{
			name: "InvalidBasicName",
			args: testArgs{
				validator:  *regexp.MustCompile("^[a-z0-9_-]+$"),
				value:      "invalid-datacenter>",
				allowEmpty: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateString(tt.args.validator, tt.args.value, tt.args.allowEmpty); (err != nil) != tt.wantErr {
				t.Errorf("ValidateString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
