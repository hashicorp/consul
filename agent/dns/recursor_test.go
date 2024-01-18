// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"strings"
	"testing"
)

// Test_handle cases are covered by the integration tests in agent/dns_test.go.
// They should be moved here when the V1 DNS server is deprecated.
//func Test_handle(t *testing.T) {

func Test_formatRecursorAddress(t *testing.T) {
	t.Parallel()
	addr, err := formatRecursorAddress("8.8.8.8")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if addr != "8.8.8.8:53" {
		t.Fatalf("bad: %v", addr)
	}
	addr, err = formatRecursorAddress("2001:4860:4860::8888")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if addr != "[2001:4860:4860::8888]:53" {
		t.Fatalf("bad: %v", addr)
	}
	_, err = formatRecursorAddress("1.2.3.4::53")
	if err == nil || !strings.Contains(err.Error(), "too many colons in address") {
		t.Fatalf("err: %v", err)
	}
	_, err = formatRecursorAddress("2001:4860:4860::8888:::53")
	if err == nil || !strings.Contains(err.Error(), "too many colons in address") {
		t.Fatalf("err: %v", err)
	}
}
