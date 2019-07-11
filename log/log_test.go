package log

import (
	"bytes"
	"log"
	"os"
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
	if Writer() != os.Stdout {
		t.Errorf("expected to return STDOUT %v", Writer())
	}
}

func TestSetup(t *testing.T) {
	type args struct {
		l Logger
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "success", args: args{l: &stdLogger{}}, wantErr: false},
		{name: "missing logger", args: args{l: nil}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Setup(tt.args.l); (err != nil) != tt.wantErr {
				t.Errorf("Setup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
