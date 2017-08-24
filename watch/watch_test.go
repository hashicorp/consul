package watch

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestParseBasic(t *testing.T) {
	params := makeParams(t, `{"type":"key", "datacenter":"dc2", "token":"12345", "key":"foo"}`)
	p, err := Parse(params)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Datacenter != "dc2" {
		t.Fatalf("Bad: %#v", p)
	}
	if p.Token != "12345" {
		t.Fatalf("Bad: %#v", p)
	}
	if p.Type != "key" {
		t.Fatalf("Bad: %#v", p)
	}
}

func TestParse_exempt(t *testing.T) {
	params := makeParams(t, `{"type":"key", "key":"foo", "handler": "foobar"}`)
	p, err := ParseExempt(params, []string{"handler"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Type != "key" {
		t.Fatalf("Bad: %#v", p)
	}
	ex := p.Exempt["handler"]
	if ex != "foobar" {
		t.Fatalf("bad: %v", ex)
	}
}

func TestParse_httpExempt(t *testing.T) {
	params := makeParams(t, `{"type":"key", "key":"foo", "http": "foobar"}`)
	p, err := ParseExempt(params, []string{"handler", "http"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Type != "key" {
		t.Fatalf("Bad: %#v", p)
	}
	ex := p.Exempt["http"]
	if ex != "foobar" {
		t.Fatalf("bad: %v", ex)
	}
}

func TestParse_exempt_NoHandler(t *testing.T) {
	params := makeParams(t, `{"type":"key", "key":"foo"}`)
	p, err := ParseExempt(params, []string{"handler", "http"})
	print(p)
	if err == nil {
		t.Fatalf("Expected ParseExempt to fail")
	}
}

func TestParse_exempt_TooManyHandlers(t *testing.T) {
	params := makeParams(t, `{"type":"key", "key":"foo", "handler": "foobar", "http": "barfoo"}`)
	_, err := ParseExempt(params, []string{"handler", "http"})
	if err == nil {
		t.Fatalf("Expected ParseExempt to fail")
	}
}

func makeParams(t *testing.T, s string) map[string]interface{} {
	var out map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("err: %v", err)
	}
	return out
}
