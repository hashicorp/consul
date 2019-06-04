package log

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLog(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	Infof("Test %s", "logging")
	if !strings.Contains(buf.String(), "INFO: Test logging") {
		t.Errorf("expected to contain `%s` but got %s", "INFO: Test logging", buf.String())
	}
	buf.Reset()
	Warnf("Test %s", "logging")
	if !strings.Contains(buf.String(), "WARN: Test logging") {
		t.Errorf("expected to contain `%s` but got %s", "WARN: Test logging", buf.String())
	}
	buf.Reset()
	Errorf("Test %s", "logging")
	if !strings.Contains(buf.String(), "ERROR: Test logging") {
		t.Errorf("expected to contain `%s` but got %s", "ERROR: Test logging", buf.String())
	}
}

func TestSetupLogging(t *testing.T) {
	stubLogf := func(string, ...interface{}) {}
	type args struct {
		infof  Func
		warnf  Func
		errorf Func
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "success", args: args{infof: stubLogf, warnf: stubLogf, errorf: stubLogf}, wantErr: false},
		{name: "missing info", args: args{infof: nil, warnf: stubLogf, errorf: stubLogf}, wantErr: true},
		{name: "missing warn", args: args{infof: stubLogf, warnf: nil, errorf: stubLogf}, wantErr: true},
		{name: "missing error", args: args{infof: stubLogf, warnf: stubLogf, errorf: nil}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Setup(tt.args.infof, tt.args.warnf, tt.args.errorf)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("did not expect error but got: %v", err)
				}
			}
		})
	}
}
