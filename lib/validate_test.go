package lib

import (
	"testing"
)

func Test_ValidateBasicName(t *testing.T) {
	type testArgs struct {
		field      string
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
				field:      "datacenter",
				value:      "custom-datacenter",
				allowEmpty: true,
			},
			wantErr: false,
		},
		{
			name: "invalid-dc-empty",
			args: testArgs{
				field:      "datacenter",
				value:      "",
				allowEmpty: false,
			},
			wantErr: true,
		},
		{
			name: "valid-primary_datacenter-empty",
			args: testArgs{
				field:      "primary_datacenter",
				value:      "",
				allowEmpty: true,
			},
			wantErr: false,
		},
		{
			name: "invalid-primary_datacenter",
			args: testArgs{
				field:      "primary_datacenter",
				value:      "dc#1",
				allowEmpty: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateBasicName(tt.args.field, tt.args.value, tt.args.allowEmpty); (err != nil) != tt.wantErr {
				t.Errorf("ValidateBasicName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
