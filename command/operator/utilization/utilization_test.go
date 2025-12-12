// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package utilization

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/version"
	mcli "github.com/mitchellh/cli"
)

func TestRun_SkipsPromptWithoutInput(t *testing.T) {
	ui := mcli.NewMockUi()
	bundle := `{"mode":"manual"}`

	oldMeta := version.VersionMetadata
	version.VersionMetadata = "ent"
	defer func() { version.VersionMetadata = oldMeta }()

	var capturedPath string
	var capturedQuery url.Values

	client := fakeClient(t, bundle, func(r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.Query()
	})

	cmd := New(ui)
	cmd.clientFn = func() (*api.Client, error) { return client, nil }

	outputPath := filepath.Join(t.TempDir(), "bundle.json")

	if code := cmd.Run([]string{"-output", outputPath}); code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}

	if capturedPath != "/v1/operator/utilization" {
		t.Fatalf("unexpected path %q", capturedPath)
	}
	if capturedQuery != nil {
		if got := capturedQuery.Get("send_report"); got != "" {
			t.Fatalf("expected send_report to be empty, got %q", got)
		}
	}

	if warn := ui.ErrorWriter.String(); !strings.Contains(warn, "skipping send prompt") {
		t.Fatalf("expected warning about skipping prompt, got %q", warn)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != bundle {
		t.Fatalf("unexpected bundle contents: %q", string(data))
	}
}

func TestRun_PromptSendsReportWhenConfirmed(t *testing.T) {
	ui := mcli.NewMockUi()
	ui.InputReader = strings.NewReader("y\n")
	bundle := `{"mode":"manual","snapshots":[]}`

	oldMeta := version.VersionMetadata
	version.VersionMetadata = "ent"
	defer func() { version.VersionMetadata = oldMeta }()

	var capturedPath string
	var capturedQuery url.Values

	client := fakeClient(t, bundle, func(r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.Query()
	})

	cmd := New(ui)
	cmd.clientFn = func() (*api.Client, error) { return client, nil }

	outputPath := filepath.Join(t.TempDir(), "bundle.json")
	args := []string{"--today-only", "-message", "manual-test", "-output", outputPath}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}

	if capturedPath != "/v1/operator/utilization" {
		t.Fatalf("unexpected path %q", capturedPath)
	}
	if capturedQuery == nil {
		t.Fatal("expected query parameters to be captured")
	}
	if got := capturedQuery.Get("send_report"); got != "true" {
		t.Fatalf("expected send_report=true, got %q", got)
	}
	if got := capturedQuery.Get("today_only"); got != "true" {
		t.Fatalf("expected today_only=true, got %q", got)
	}
	if got := capturedQuery.Get("message"); got != "manual-test" {
		t.Fatalf("expected message=manual-test, got %q", got)
	}

	promptOutput := ui.OutputWriter.String()
	if !strings.Contains(promptOutput, "Send usage report") {
		t.Fatalf("expected prompt to be written, got %q", promptOutput)
	}
	if !strings.Contains(promptOutput, "Usage report sent to HashiCorp.") {
		t.Fatalf("expected confirmation output, got %q", promptOutput)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != bundle {
		t.Fatalf("unexpected bundle contents: %q", string(data))
	}
}

func TestRun_CommunityEditionReturnsFriendlyError(t *testing.T) {
	ui := mcli.NewMockUi()

	oldMeta := version.VersionMetadata
	version.VersionMetadata = ""
	defer func() { version.VersionMetadata = oldMeta }()

	cmd := New(ui)
	cmd.clientFn = func() (*api.Client, error) {
		t.Fatal("client should not be called in community edition")
		return nil, nil
	}

	if code := cmd.Run(nil); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	msg := ui.ErrorWriter.String()
	if !strings.Contains(msg, "requires Consul Enterprise") {
		t.Fatalf("expected enterprise warning, got %q", msg)
	}
	if ui.OutputWriter.String() != "" {
		t.Fatalf("expected no output, got %q", ui.OutputWriter.String())
	}
}

func fakeClient(t *testing.T, body string, capture func(*http.Request)) *api.Client {
	t.Helper()

	cfg := api.DefaultConfig()
	cfg.Address = "example.invalid"
	cfg.HttpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if capture != nil {
				capture(r)
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}
			return resp, nil
		}),
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		t.Fatalf("creating API client: %v", err)
	}

	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
