package log

import (
	"bytes"
	"context"
	golog "log"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

type p struct{}

func (p p) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return 0, nil
}

func (p p) Name() string { return "testplugin" }

func TestPlugins(t *testing.T) {
	var f bytes.Buffer
	const ts = "test"
	golog.SetOutput(&f)

	lg := NewWithPlugin(p{})

	lg.Info(ts)
	if x := f.String(); !strings.Contains(x, "plugin/testplugin") {
		t.Errorf("Expected log to be %s, got %s", info+ts, x)
	}
}
