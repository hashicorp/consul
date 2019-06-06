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
	Printf("Test %s", "logging")
	if !strings.Contains(buf.String(), "Test logging") {
		t.Errorf("expected to contain `%s` but got %s", "Test logging", buf.String())
	}
	buf.Reset()
	Println("Test logging")
	if !strings.Contains(buf.String(), "Test logging") {
		t.Errorf("expected to contain `%s` but got %s", "Test logging", buf.String())
	}	
}

func TestSetupLogging(t *testing.T) {
	stubPrintf := func(string, ...interface{}) {}
	stubPrintln := func( ...interface{}) {}
	type args struct {
		printf  Funcf
		println  Funcln
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "success", args: args{printf: stubPrintf, println: stubPrintln}, wantErr: false},
		{name: "missing info", args: args{printf: nil, println: stubPrintln}, wantErr: true},
		{name: "missing warn", args: args{printf: stubPrintf, println: nil}, wantErr: true},		
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Setup(tt.args.printf, tt.args.println)
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
