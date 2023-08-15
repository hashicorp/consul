// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBasic(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestParseHttpHandlerConfig_hcl(t *testing.T) {
	t.Parallel()

	testFilePath := "testdata/watches.hcl"
	data, err := os.ReadFile(testFilePath)
	require.NoError(t, err)

	fs := config.FileSource{Name: testFilePath, Data: string(data), Format: "hcl"}
	c, _, err := fs.Parse() //parse hcl and get config
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Without WeakDecode and http header having key (x-key) with
	// single element slice (["foo/bar/baz"]) returns error.
	// Reason - without WeakDecode single element slice is decoded as string instead of to slice
	_, err = parseHttpHandlerConfig(c.Watches[0]["http_handler_config"], false)
	assert.NotNil(t, err)

	// Use WeakDecode
	hc, err := parseHttpHandlerConfig(c.Watches[0]["http_handler_config"], true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hc.Header["x-key"][0] != "foo/bar/baz" {
		t.Fatalf("Bad: %v", hc.Header)
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
