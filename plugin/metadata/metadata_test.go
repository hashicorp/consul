package metadata

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

type testProvider map[string]Func

func (tp testProvider) Metadata(ctx context.Context, state request.Request) context.Context {
	for k, v := range tp {
		SetValueFunc(ctx, k, v)
	}
	return ctx
}

type testHandler struct{ ctx context.Context }

func (m *testHandler) Name() string { return "test" }

func (m *testHandler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	m.ctx = ctx
	return 0, nil
}

func TestMetadataServeDNS(t *testing.T) {
	expectedMetadata := []testProvider{
		{"test/key1": func() string { return "testvalue1" }},
		{"test/key2": func() string { return "two" }, "test/key3": func() string { return "testvalue3" }},
	}
	// Create fake Providers based on expectedMetadata
	providers := []Provider{}
	for _, e := range expectedMetadata {
		providers = append(providers, e)
	}

	next := &testHandler{} // fake handler which stores the resulting context
	m := Metadata{
		Zones:     []string{"."},
		Providers: providers,
		Next:      next,
	}

	ctx := context.TODO()
	m.ServeDNS(ctx, &test.ResponseWriter{}, new(dns.Msg))
	nctx := next.ctx

	for _, expected := range expectedMetadata {
		for label, expVal := range expected {
			if !IsLabel(label) {
				t.Errorf("Expected label %s is not considered a valid label", label)
			}
			val := ValueFunc(nctx, label)
			if val() != expVal() {
				t.Errorf("Expected value %s for %s, but got %s", expVal(), label, val())
			}
		}
	}
}

func TestLabelFormat(t *testing.T) {
	labels := []struct {
		label   string
		isValid bool
	}{
		// ok
		{"plugin/LABEL", true},
		{"p/LABEL", true},
		{"plugin/L", true},
		// fails
		{"LABEL", false},
		{"plugin.LABEL", false},
		{"/NO-PLUGIN-NOT-ACCEPTED", false},
		{"ONLY-PLUGIN-NOT-ACCEPTED/", false},
		{"PLUGIN/LABEL/SUB-LABEL", false},
		{"/", false},
		{"//", false},
	}

	for _, test := range labels {
		if x := IsLabel(test.label); x != test.isValid {
			t.Errorf("Label %v expected %v, got: %v", test.label, test.isValid, x)
		}
	}
}
