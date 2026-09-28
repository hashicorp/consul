// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const (
	testSource = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testTarget = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testMoved  = "cccccccccccccccccccccccccccccccccccccccc"
)

const testManifest = `
schema = 1
target_repository = "hashicorp/consul-enterprise"
active_versions {
  version "1.2" {
    ce_active = true
  }
  version "1.22" {
    ce_active = true
  }
  version "1.23" {
    ce_active = false
    lts = true
  }
}
`

func testJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func testLabels(names ...string) []label {
	result := make([]label, 0, len(names))
	for _, name := range names {
		result = append(result, label{Name: name})
	}
	return result
}

type fakeBackporter struct {
	t             *testing.T
	b             backporter
	pr            map[string]any
	repo          map[string]any
	manifest      string
	diff          []byte
	open          []pullRequest
	requests      []string
	fetched       []string
	created       [][]byte
	probed        [][2]string
	completed     []string
	prReads       int
	manifestReads int
	openReads     int
	getHook       func(path, accept string) ([]byte, error, bool)
	fetchHook     func(ref string) (string, error)
	completeHook  func(source string, diff []byte) error
	presentHook   func(source, target string) (bool, error)
	createHook    func(payload []byte) error
}

func newFakeBackporter(t *testing.T, labels ...string) *fakeBackporter {
	t.Helper()
	f := &fakeBackporter{
		t: t, manifest: testManifest, diff: []byte("diff --git a/file b/file\nmock aggregate diff\n"),
		open: []pullRequest{},
		repo: map[string]any{
			"full_name": repository, "name": "consul", "default_branch": "main",
			"owner": map[string]any{"login": "hashicorp", "id": 761456, "type": "Organization"},
			"id":    123456,
		},
		pr: map[string]any{
			"number": 123, "merged": true, "merge_commit_sha": testSource,
			"title": "Preserve all metadata $(do-not-execute)",
			"body":  "Original body with \"quotes\"\nand `commands`",
			"state": "closed", "html_url": "https://github.com/hashicorp/consul/pull/123",
			"user":                map[string]any{"login": "author", "id": 42},
			"assignees":           []any{map[string]any{"login": "assignee", "id": 43}},
			"requested_reviewers": []any{map[string]any{"login": "reviewer", "id": 44}},
			"requested_teams":     []any{map[string]any{"slug": "team", "id": 45}},
			"milestone":           map[string]any{"title": "next", "number": 7},
			"unknown_metadata":    map[string]any{"must_survive": []any{1, true, "value"}},
			"base": map[string]any{
				"ref": "main", "sha": testTarget,
				"repo": map[string]any{"full_name": repository, "owner": map[string]any{"login": "hashicorp"}},
			},
			"head": map[string]any{
				"ref": "feature", "sha": testSource,
				"repo": map[string]any{"full_name": "contributor/consul", "owner": map[string]any{"login": "contributor"}},
			},
			"labels": testLabels(labels...),
		},
	}
	f.b = backporter{
		get: f.get,
		fetch: func(ref string) (string, error) {
			f.fetched = append(f.fetched, ref)
			if f.fetchHook != nil {
				return f.fetchHook(ref)
			}
			if ref == f.pr["merge_commit_sha"] {
				return ref, nil
			}
			return testTarget, nil
		},
		complete: func(source string, diff []byte) error {
			f.completed = append(f.completed, source)
			if source != f.pr["merge_commit_sha"] || !bytes.Equal(diff, f.diff) {
				t.Fatalf("complete received source %q, diff %q", source, diff)
			}
			if f.completeHook != nil {
				return f.completeHook(source, diff)
			}
			return nil
		},
		present: func(source, target string) (bool, error) {
			f.probed = append(f.probed, [2]string{source, target})
			if f.presentHook != nil {
				return f.presentHook(source, target)
			}
			return false, nil
		},
		create: func(payload []byte) error {
			f.created = append(f.created, bytes.Clone(payload))
			if f.createHook != nil {
				return f.createHook(payload)
			}
			return nil
		},
		log: io.Discard,
	}
	return f
}

func (f *fakeBackporter) get(path, accept string) ([]byte, error) {
	f.t.Helper()
	f.requests = append(f.requests, path+" "+accept)
	if path == "repos/"+repository+"/pulls/123" && accept == "" {
		f.prReads++
	}
	if strings.Contains(path, "/contents/") {
		f.manifestReads++
	}
	if strings.Contains(path, "/pulls?") {
		f.openReads++
	}
	if f.getHook != nil {
		if data, err, handled := f.getHook(path, accept); handled {
			return data, err
		}
	}
	switch {
	case path == "repos/"+repository:
		return testJSON(f.t, f.repo), nil
	case path == "repos/"+repository+"/pulls/123" && accept == "":
		return testJSON(f.t, f.pr), nil
	case path == "repos/"+repository+"/pulls/123" && accept == "application/vnd.github.diff":
		return f.diff, nil
	case strings.HasPrefix(path, "repos/"+repository+"/contents/.release/versions.hcl?"):
		u, err := url.Parse(path)
		if err != nil || u.Query().Get("ref") != f.repo["default_branch"] ||
			path != "repos/"+repository+"/contents/.release/versions.hcl?ref="+url.QueryEscape(f.repo["default_branch"].(string)) ||
			accept != "application/vnd.github.raw+json" {
			f.t.Fatalf("manifest request did not use current default branch/raw media type: %q %q", path, accept)
		}
		return []byte(f.manifest), nil
	case strings.HasPrefix(path, "repos/"+repository+"/pulls?"):
		u, err := url.Parse(path)
		if err != nil || u.Query().Get("state") != "open" || u.Query().Get("per_page") != "100" ||
			u.Query().Get("page") != "1" || !strings.HasPrefix(u.Query().Get("base"), "release/") || accept != "" {
			f.t.Fatalf("unexpected open PR query %q %q", path, accept)
		}
		return testJSON(f.t, f.open), nil
	default:
		f.t.Fatalf("unexpected API request %q %q", path, accept)
		return nil, errors.New("unexpected API request")
	}
}

func (f *fakeBackporter) event(action, selectedLabel string) event {
	f.t.Helper()
	data := testJSON(f.t, map[string]any{
		"action": action, "number": 123, "repository": f.repo, "pull_request": f.pr,
		"label":  map[string]any{"name": selectedLabel},
		"sender": map[string]any{"login": "event-sender", "id": 99, "type": "User"},
		"installation": map[string]any{
			"id": 987, "node_id": "installation-metadata",
		},
	})
	e, err := parseEvent(data)
	if err != nil {
		f.t.Fatal(err)
	}
	return e
}

func (f *fakeBackporter) createdLabels() []string {
	result := make([]string, 0, len(f.created))
	for _, payload := range f.created {
		var e event
		if err := json.Unmarshal(payload, &e); err != nil {
			f.t.Fatal(err)
		}
		if e.Action != "labeled" {
			f.t.Errorf("creator action = %q, want labeled", e.Action)
		}
		result = append(result, e.Label.Name)
	}
	return result
}

func TestParseEvent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		valid  bool
	}{
		{"valid", func(map[string]any) {}, true},
		{"wrong repository", func(m map[string]any) { m["repository"] = map[string]any{"full_name": "fork/consul"} }, false},
		{"missing repository", func(m map[string]any) { delete(m, "repository") }, false},
		{"zero number", func(m map[string]any) { m["number"] = 0 }, false},
		{"negative number", func(m map[string]any) { m["number"] = -123 }, false},
		{"number mismatch", func(m map[string]any) { m["number"] = 124 }, false},
		{"string number", func(m map[string]any) { m["number"] = "123" }, false},
		{"missing pull request", func(m map[string]any) { delete(m, "pull_request") }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t)
			input := testObject(t, testJSON(t, f.event("closed", "")))
			tc.mutate(input)
			e, err := parseEvent(testJSON(t, input))
			if (err == nil) != tc.valid {
				t.Fatalf("parseEvent error = %v, valid = %v", err, tc.valid)
			}
			if tc.valid && e.raw["pull_request"] == nil {
				t.Fatal("raw event metadata was not retained")
			}
		})
	}
	for _, input := range []string{"", "{", "null", "[]"} {
		if _, err := parseEvent([]byte(input)); err == nil {
			t.Errorf("parseEvent(%q) succeeded", input)
		}
	}
}

func TestReadManifest(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"schema one and enterprise metadata", testManifest, true},
		{"empty versions", "schema = 1\nactive_versions {}", true},
		{"optional flags", "schema = 1\nactive_versions {\n version \"1.2.3\" {}\n}", true},
		{"missing schema", "active_versions {}", false},
		{"unsupported schema", strings.Replace(testManifest, "schema = 1", "schema = 2", 1), false},
		{"missing active block", "schema = 1", false},
		{"malformed HCL", "schema = {", false},
		{"unknown attribute", testManifest + "\nunexpected = true", false},
		{"duplicate version", strings.Replace(testManifest, `"1.22"`, `"1.2"`, 1), false},
		{"non numeric", strings.Replace(testManifest, `"1.2"`, `"latest"`, 1), false},
		{"version prefix", strings.Replace(testManifest, `"1.2"`, `"v1.2"`, 1), false},
		{"too many components", strings.Replace(testManifest, `"1.2"`, `"1.2.3.4"`, 1), false},
		{"non boolean flag", strings.Replace(testManifest, "ce_active = true", "ce_active = 1", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readManifest([]byte(tc.data))
			if (err == nil) != tc.valid {
				t.Fatalf("readManifest error = %v, valid = %v", err, tc.valid)
			}
		})
	}
}

func TestLoadManifestUsesCurrentDefaultBranchAPI(t *testing.T) {
	for _, branch := range []string{"main", "default/new", "release+candidate#1"} {
		t.Run(branch, func(t *testing.T) {
			f := newFakeBackporter(t)
			f.repo["default_branch"] = branch
			m, err := f.b.loadManifest(branch)
			if err != nil || !m.active("1.2") {
				t.Fatalf("loadManifest = %#v, %v", m, err)
			}
			want := "repos/" + repository + "/contents/.release/versions.hcl?ref=" + url.QueryEscape(branch) + " application/vnd.github.raw+json"
			if !slices.Equal(f.requests, []string{want}) || len(f.fetched) != 0 {
				t.Fatalf("manifest must use only the current default-branch contents API: requests=%v fetches=%v", f.requests, f.fetched)
			}
		})
	}
}

func TestManifestActive(t *testing.T) {
	for _, tc := range []struct {
		name, requested string
		lines           []releaseLine
		want            bool
	}{
		{"exact active", "1.2", []releaseLine{{Version: "1.2", CEActive: true}}, true},
		{"exact inactive", "1.2", []releaseLine{{Version: "1.2"}}, false},
		{"LTS does not activate CE", "1.2", []releaseLine{{Version: "1.2", LTS: true}}, false},
		{"dotted patch prefix", "1.2", []releaseLine{{Version: "1.2.9", CEActive: true}}, true},
		{"not a decimal prefix", "1.2", []releaseLine{{Version: "1.22", CEActive: true}}, false},
		{"not a patch decimal prefix", "1.2", []releaseLine{{Version: "1.22.1", CEActive: true}}, false},
		{"any active patch", "1.2", []releaseLine{{Version: "1.2.1"}, {Version: "1.2.2", CEActive: true}}, true},
		{"inactive exact overrides patch", "1.2", []releaseLine{{Version: "1.2.3", CEActive: true}, {Version: "1.2"}}, false},
		{"active exact overrides inactive patch", "1.2", []releaseLine{{Version: "1.2.3"}, {Version: "1.2", CEActive: true}}, true},
		{"missing version", "1.3", []releaseLine{{Version: "1.2", CEActive: true}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var m manifest
			m.Active.Versions = tc.lines
			if got := m.active(tc.requested); got != tc.want {
				t.Fatalf("active(%q) = %v, want %v", tc.requested, got, tc.want)
			}
		})
	}
}

func TestTargets(t *testing.T) {
	m, err := readManifest([]byte(testManifest))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		labels []string
		want   []string
	}{
		{"deduplicated and sorted", []string{"backport/1.22", "backport/1.2", "backport/1.22"}, []string{"1.2", "1.22"}},
		{"inactive LTS", []string{"backport/1.23"}, nil},
		{"unsupported label formats", []string{"bug", "backport/all", "backport/ent/1.2", "backport/1.2.3", "backport/1.2.x", "backport/1.2suffix", " backport/1.2", "backport/1.2\n"}, nil},
		{"unknown version", []string{"backport/9.9"}, nil},
		{"no labels", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := targets(pullRequest{Labels: testLabels(tc.labels...)}, m)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("targets = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNeedsUmbrellaLabels(t *testing.T) {
	m, err := readManifest([]byte(testManifest))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		labels []string
		want   bool
	}{
		{"no umbrella", []string{"backport/1.2"}, false},
		{"unexpanded", []string{"backport/all"}, true},
		{"CE missing", []string{"backport/all", "backport/1.2", "backport/ent/1.23"}, true},
		{"Enterprise missing", []string{"backport/all", "backport/1.2", "backport/1.22"}, true},
		{"wrong inactive prefix", []string{"backport/all", "backport/1.2", "backport/1.22", "backport/1.23"}, true},
		{"fully expanded", []string{"backport/all", "backport/1.2", "backport/1.22", "backport/ent/1.23"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsUmbrellaLabels(pullRequest{Labels: testLabels(tc.labels...)}, m); got != tc.want {
				t.Fatalf("needsUmbrellaLabels = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunEventSelection(t *testing.T) {
	for _, tc := range []struct {
		name, action, eventLabel string
		labels                   []string
		mutate                   func(*fakeBackporter, *event)
		want                     []string
		noAPI                    bool
		noManifest               bool
	}{
		{name: "closed reconciles all labels", action: "closed", labels: []string{"backport/1.22", "backport/1.2", "backport/1.2", "bug"}, want: []string{"backport/1.2", "backport/1.22"}},
		{name: "labeled reconciles all labels", action: "labeled", eventLabel: "backport/1.22", labels: []string{"backport/1.2", "backport/1.22"}, want: []string{"backport/1.2", "backport/1.22"}},
		{name: "inactive CE label still reconciles current requests", action: "labeled", eventLabel: "backport/1.23", labels: []string{"backport/1.2", "backport/1.23"}, want: []string{"backport/1.2"}},
		{name: "unrelated label event reconciles current requests", action: "labeled", eventLabel: "bug", labels: []string{"backport/1.2"}, want: []string{"backport/1.2"}},
		{name: "Enterprise label event reconciles current CE requests", action: "labeled", eventLabel: "backport/ent/1.2", labels: []string{"backport/1.2"}, want: []string{"backport/1.2"}},
		{name: "invalid target label event reconciles current requests", action: "labeled", eventLabel: "backport/1.2.3", labels: []string{"backport/1.2"}, want: []string{"backport/1.2"}},
		{name: "unlabeled ignored", action: "unlabeled", eventLabel: "backport/1.2", labels: []string{"backport/1.22"}, noAPI: true},
		{name: "opened ignored", action: "opened", labels: []string{"backport/1.2"}, noAPI: true},
		{name: "edited ignored", action: "edited", labels: []string{"backport/1.2"}, noAPI: true},
		{name: "event unmerged", action: "closed", labels: []string{"backport/1.2"}, mutate: func(_ *fakeBackporter, e *event) { e.PullRequest.Merged = false }, noAPI: true},
		{name: "fresh unmerged", action: "closed", labels: []string{"backport/1.2"}, mutate: func(f *fakeBackporter, _ *event) { f.pr["merged"] = false }},
		{name: "nondefault base", action: "closed", labels: []string{"backport/1.2"}, mutate: func(f *fakeBackporter, _ *event) { f.pr["base"].(map[string]any)["ref"] = "release/1.22.x" }},
		{name: "inactive only", action: "closed", labels: []string{"backport/1.23"}},
		{name: "unrelated labels only", action: "closed", labels: []string{"bug", "backport/ent/1.2"}, noManifest: true},
		{name: "unrelated event with no CE requests", action: "labeled", eventLabel: "bug", labels: []string{"bug", "backport/ent/1.2"}, noManifest: true},
		{name: "malformed target labels only", action: "labeled", eventLabel: "backport/1.2.3", labels: []string{"backport/1.2.3", "backport/1.2.x"}, noManifest: true},
		{name: "no labels", action: "closed", noManifest: true},
		{name: "fresh labels remove stale target", action: "labeled", eventLabel: "backport/1.2", labels: []string{"backport/1.2"}, mutate: func(f *fakeBackporter, _ *event) { f.pr["labels"] = testLabels() }, noManifest: true},
		{name: "fresh labels replace stale target", action: "labeled", eventLabel: "backport/1.2", labels: []string{"backport/1.2"}, mutate: func(f *fakeBackporter, _ *event) { f.pr["labels"] = testLabels("backport/1.22") }, want: []string{"backport/1.22"}},
		{name: "renamed default branch", action: "closed", labels: []string{"backport/1.2"}, mutate: func(f *fakeBackporter, _ *event) {
			f.repo["default_branch"] = "default/new"
			f.pr["base"].(map[string]any)["ref"] = "default/new"
		}, want: []string{"backport/1.2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t, tc.labels...)
			e := f.event(tc.action, tc.eventLabel)
			if tc.mutate != nil {
				tc.mutate(f, &e)
			}
			if err := f.b.run(e); err != nil {
				t.Fatal(err)
			}
			if got := f.createdLabels(); !slices.Equal(got, tc.want) {
				t.Fatalf("created %v, want %v", got, tc.want)
			}
			if tc.noAPI && len(f.requests) != 0 {
				t.Errorf("irrelevant event made API calls: %v", f.requests)
			}
			if tc.noManifest && f.manifestReads != 0 {
				t.Errorf("event without current CE/all labels loaded the manifest %d times", f.manifestReads)
			}
			if len(tc.want) == 0 && (len(f.fetched) != 0 || len(f.completed) != 0 || len(f.probed) != 0) {
				t.Errorf("ignored event reached Git: fetch=%v complete=%v present=%v", f.fetched, f.completed, f.probed)
			}
			if len(tc.want) > 0 && len(f.completed) != 1 {
				t.Errorf("source validated %d times, want once for all targets", len(f.completed))
			}
			if len(tc.want) > 0 {
				wantFetches := []string{testSource}
				for _, name := range tc.want {
					ref := "refs/heads/release/" + strings.TrimPrefix(name, "backport/") + ".x"
					wantFetches = append(wantFetches, ref, ref)
				}
				if !slices.Equal(f.fetched, wantFetches) {
					t.Errorf("fetches = %v, want only source and requested release branches %v", f.fetched, wantFetches)
				}
			}
		})
	}
}

func TestRunCoalescedUnrelatedNewestEventReconcilesCurrentRequests(t *testing.T) {
	f := newFakeBackporter(t, "bug")
	e := f.event("labeled", "bug")
	// Only the newest event runs; earlier queued backport-label events are lost.
	f.pr["labels"] = testLabels("bug", "backport/1.22", "backport/1.2", "backport/1.23", "backport/ent/1.2")
	f.createHook = func(payload []byte) error {
		assertCreatorMetadata(t, payload, e, f.pr, f.repo)
		return nil
	}
	if err := f.b.run(e); err != nil {
		t.Fatal(err)
	}
	if got, want := f.createdLabels(), []string{"backport/1.2", "backport/1.22"}; !slices.Equal(got, want) {
		t.Fatalf("newest unrelated event lost coalesced requests: created=%v, want=%v", got, want)
	}
	if len(f.completed) != 1 || len(f.probed) != 2 {
		t.Fatalf("coalesced requests bypassed source/target guards: complete=%v present=%v", f.completed, f.probed)
	}
}

func assertCreatorMetadata(t *testing.T, payload []byte, e event, pr, repo map[string]any) {
	t.Helper()
	got := testObject(t, payload)
	wantPR := testObject(t, testJSON(t, pr))
	wantRepo := testObject(t, testJSON(t, repo))
	if !reflect.DeepEqual(got["pull_request"], wantPR) {
		t.Errorf("creator did not preserve complete current PR metadata\n got: %#v\nwant: %#v", got["pull_request"], wantPR)
	}
	if !reflect.DeepEqual(got["repository"], wantRepo) {
		t.Errorf("creator did not preserve repository owner/full metadata: %#v", got["repository"])
	}
	for key, value := range e.raw {
		if key == "action" || key == "label" || key == "repository" || key == "pull_request" {
			continue
		}
		var want any
		if err := json.Unmarshal(value, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got[key], want) {
			t.Errorf("creator lost event %s: got %#v, want %#v", key, got[key], want)
		}
	}
}

func TestRunUmbrellaExpansionAndMetadata(t *testing.T) {
	f := newFakeBackporter(t, "backport/all", "bug")
	e := f.event("closed", "")
	f.createHook = func(payload []byte) error {
		assertCreatorMetadata(t, payload, e, f.pr, f.repo)
		got := testObject(t, payload)
		if got["action"] != "labeled" {
			t.Fatal("umbrella must be a labeled event, never a closed creator event")
		}
		if got["label"].(map[string]any)["name"] == "backport/all" {
			if len(f.fetched) != 0 {
				t.Fatal("umbrella expansion should precede per-target Git checks")
			}
			f.pr["labels"] = testLabels("backport/all", "bug", "backport/1.2", "backport/1.22", "backport/ent/1.23")
			f.pr["post_expansion_metadata"] = "retain the refreshed PR"
		} else if f.prReads < 2 {
			t.Fatal("creator did not refresh labels after expansion")
		}
		return nil
	}
	if err := f.b.run(e); err != nil {
		t.Fatal(err)
	}
	if got, want := f.createdLabels(), []string{"backport/all", "backport/1.2", "backport/1.22"}; !slices.Equal(got, want) {
		t.Fatalf("created labels %v, want %v", got, want)
	}
	if len(f.completed) != 1 || len(f.probed) != 2 {
		t.Fatalf("per-target guards not applied: completed=%v probed=%v", f.completed, f.probed)
	}
}

func TestRunAlreadyExpandedUmbrellaUsesGuard(t *testing.T) {
	for _, present := range []bool{true, false} {
		t.Run(fmt.Sprintf("present=%v", present), func(t *testing.T) {
			f := newFakeBackporter(t, "backport/all", "backport/1.2", "backport/1.22", "backport/ent/1.23")
			e := f.event("labeled", "backport/all")
			f.presentHook = func(string, string) (bool, error) { return present, nil }
			f.createHook = func(payload []byte) error {
				assertCreatorMetadata(t, payload, e, f.pr, f.repo)
				return nil
			}
			if err := f.b.run(e); err != nil {
				t.Fatal(err)
			}
			want := []string{"backport/1.2", "backport/1.22"}
			if present {
				want = nil
			}
			if got := f.createdLabels(); !slices.Equal(got, want) {
				t.Fatalf("creator labels %v, want %v; umbrella must never bypass guard", got, want)
			}
			if len(f.completed) != 1 || len(f.probed) != 2 {
				t.Fatalf("missing guard calls: complete=%v present=%v", f.completed, f.probed)
			}
		})
	}
}

func TestRunCoalescedUnrelatedEventHandlesCurrentUmbrella(t *testing.T) {
	expandedLabels := []string{"backport/all", "backport/1.2", "backport/1.22", "backport/ent/1.23"}
	for _, tc := range []struct {
		name       string
		labels     []string
		wantExpand bool
		present    bool
		existing   bool
	}{
		{"only umbrella needs backports", []string{"backport/all"}, true, false, false},
		{"only umbrella content already present", []string{"backport/all"}, true, true, false},
		{"Enterprise labels missing", []string{"backport/all", "backport/1.2", "backport/1.22"}, true, false, false},
		{"fully expanded needs backports", expandedLabels, false, false, false},
		{"fully expanded content already present", expandedLabels, false, true, false},
		{"fully expanded backports already open", expandedLabels, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t, "bug")
			e := f.event("labeled", "bug")
			// The unrelated newest event replaces the queued backport/all event.
			f.pr["labels"] = testLabels(tc.labels...)
			if tc.existing {
				f.open = []pullRequest{
					testOpenBackport(123, "release/1.2.x"),
					testOpenBackport(123, "release/1.22.x"),
				}
			}
			f.presentHook = func(string, string) (bool, error) { return tc.present, nil }
			f.createHook = func(payload []byte) error {
				assertCreatorMetadata(t, payload, e, f.pr, f.repo)
				var delegated event
				if err := json.Unmarshal(payload, &delegated); err != nil {
					t.Fatal(err)
				}
				if delegated.Action != "labeled" || !hasLabel(delegated.PullRequest, "backport/all") {
					t.Fatal("delegation must be labeled and retain the original umbrella PR label")
				}
				if delegated.Label.Name == "backport/all" {
					if !tc.wantExpand || len(f.created) != 1 {
						t.Fatal("fully expanded umbrella was delegated again")
					}
					if len(f.fetched)+len(f.completed)+len(f.probed) != 0 {
						t.Fatal("label-only expansion should precede per-target Git checks")
					}
					f.pr["labels"] = testLabels(expandedLabels...)
					f.pr["expansion_metadata"] = "preserve refreshed metadata"
				} else {
					if !labelPattern.MatchString(delegated.Label.Name) || len(f.completed) != 1 || len(f.probed) == 0 {
						t.Fatal("ordinary creator must receive an exact target label after the guards")
					}
					if tc.wantExpand && f.prReads < 3 {
						t.Fatal("ordinary creator did not refresh the PR after umbrella expansion")
					}
				}
				return nil
			}
			if err := f.b.run(e); err != nil {
				t.Fatal(err)
			}
			var want []string
			if tc.wantExpand {
				want = append(want, "backport/all")
			}
			if !tc.present && !tc.existing {
				want = append(want, "backport/1.2", "backport/1.22")
			}
			if got := f.createdLabels(); !slices.Equal(got, want) {
				t.Fatalf("coalesced umbrella delegations = %v, want %v", got, want)
			}
			if tc.existing {
				if len(f.fetched)+len(f.completed)+len(f.probed) != 0 {
					t.Fatal("existing backports should bypass Git, not the open-PR guard")
				}
			} else if len(f.completed) != 1 || len(f.probed) != 2 {
				t.Fatalf("umbrella bypassed per-target guards: complete=%v present=%v", f.completed, f.probed)
			}
		})
	}
}

func TestRunCreatorUsesLastRefreshedMetadata(t *testing.T) {
	f := newFakeBackporter(t, "backport/1.2")
	e := f.event("labeled", "backport/1.2")
	f.repo["owner"] = map[string]any{"login": "hashicorp", "id": 123, "fresh_owner_metadata": true}
	f.getHook = func(path, accept string) ([]byte, error, bool) {
		if path == "repos/"+repository+"/pulls/123" && accept == "" && f.prReads == 3 {
			f.pr["title"] = "Refreshed title"
			f.pr["body"] = "Refreshed body"
			f.pr["labels"] = []any{
				map[string]any{"name": "backport/1.2", "id": 11, "color": "abcdef", "description": "target"},
				map[string]any{"name": "bug", "id": 12, "node_id": "label-id", "default": false},
			}
			f.pr["user"] = map[string]any{"login": "fresh-author", "id": 77, "node_id": "user-id"}
			f.pr["new_field"] = []any{"must", "be", "retained"}
		}
		return nil, nil, false
	}
	original := testJSON(t, e.raw)
	f.createHook = func(payload []byte) error {
		assertCreatorMetadata(t, payload, e, f.pr, f.repo)
		return nil
	}
	if err := f.b.run(e); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.createdLabels(), []string{"backport/1.2"}) {
		t.Fatalf("creator labels = %v", f.createdLabels())
	}
	if !bytes.Equal(e.raw["pull_request"], testObjectRaw(t, original, "pull_request")) ||
		!bytes.Equal(e.raw["label"], testObjectRaw(t, original, "label")) {
		t.Fatal("payload creation mutated original event PR or label")
	}
}

func testObjectRaw(t *testing.T, data []byte, key string) json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	return object[key]
}

func testOpenBackport(number int, branch string) pullRequest {
	var pr pullRequest
	pr.Number = 900
	pr.Body = fmt.Sprintf("\n## Backport\n\nThis PR is auto-generated from #%d to be assessed for backporting due to the inclusion of the label backport/1.2.\n\nOther text", number)
	pr.Base.Ref = branch
	pr.Base.Repo.FullName = repository
	pr.Head.Ref = "backport/1.2/123"
	pr.Head.Repo.FullName = repository
	return pr
}

func TestExistingBackportProvenance(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*pullRequest)
		want   bool
	}{
		{"pinned creator body", func(*pullRequest) {}, true},
		{"surrounding whitespace", func(pr *pullRequest) { pr.Body = "\n \t" + pr.Body + " \n" }, true},
		{"different source number", func(pr *pullRequest) { pr.Body = strings.Replace(pr.Body, "#123 ", "#1234 ", 1) }, false},
		{"bare number in body", func(pr *pullRequest) { pr.Body = "This fixes #123" }, false},
		{"provenance quoted later", func(pr *pullRequest) { pr.Body = "Quoted unrelated PR:\n" + pr.Body }, false},
		{"different source copies matching provenance", func(pr *pullRequest) {
			pr.Body = testOpenBackport(456, pr.Base.Ref).Body + "\n\nOriginal PR body:\n" + pr.Body
		}, false},
		{"lookalike source copies matching provenance", func(pr *pullRequest) {
			pr.Body = testOpenBackport(1234, pr.Base.Ref).Body + "\n\nOriginal PR body:\n" + pr.Body
		}, false},
		{"matching provenance appears only in a blockquote", func(pr *pullRequest) {
			pr.Body = "> " + strings.ReplaceAll(strings.TrimSpace(pr.Body), "\n", "\n> ")
		}, false},
		{"matching prefix with unrelated copied provenance", func(pr *pullRequest) {
			pr.Body += "\n\nOriginal PR body:\n" + testOpenBackport(456, pr.Base.Ref).Body
		}, true},
		{"unrelated body", func(pr *pullRequest) { pr.Body = "Backport #123 to 1.2" }, false},
		{"truncated provenance", func(pr *pullRequest) {
			pr.Body = "## Backport\n\nThis PR is auto-generated from #123 to be assessed for backporting"
		}, false},
		{"matching PR number only", func(pr *pullRequest) { pr.Number, pr.Body = 123, "" }, false},
		{"wrong target", func(pr *pullRequest) { pr.Base.Ref = "release/1.22.x" }, false},
		{"fork branch", func(pr *pullRequest) { pr.Head.Repo.FullName = "fork/consul" }, false},
		{"unknown head repo", func(pr *pullRequest) { pr.Head.Repo.FullName = "" }, false},
		{"ordinary native branch", func(pr *pullRequest) { pr.Head.Ref = "feature/123" }, false},
		{"lookalike branch prefix", func(pr *pullRequest) { pr.Head.Ref = "backportish/123" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t)
			pr := testOpenBackport(123, "release/1.2.x")
			tc.mutate(&pr)
			f.open = []pullRequest{pr}
			got, err := f.b.existing(123, "release/1.2.x")
			if err != nil || got != tc.want {
				t.Fatalf("existing = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	t.Run("matching title alone", func(t *testing.T) {
		f := newFakeBackporter(t)
		f.getHook = func(string, string) ([]byte, error, bool) {
			return []byte(`[{"number":900,"title":"Backport #123 to release/1.2.x","body":"unrelated","base":{"ref":"release/1.2.x"},"head":{"ref":"backport/1.2/123","repo":{"full_name":"hashicorp/consul"}}}]`), nil, true
		}
		if got, err := f.b.existing(123, "release/1.2.x"); err != nil || got {
			t.Fatalf("matching title suppressed backport: %v, %v", got, err)
		}
	})
}

func TestExistingBackportPagination(t *testing.T) {
	for _, match := range []bool{false, true} {
		t.Run(fmt.Sprintf("match_on_page_two=%v", match), func(t *testing.T) {
			f := newFakeBackporter(t)
			f.getHook = func(path, accept string) ([]byte, error, bool) {
				u, err := url.Parse(path)
				if err != nil {
					t.Fatal(err)
				}
				q := u.Query()
				if u.Path != "repos/"+repository+"/pulls" || q.Get("base") != "release/1.2.x" || q.Get("state") != "open" || q.Get("per_page") != "100" || accept != "" {
					t.Fatalf("incorrect paginated query: %q", path)
				}
				if q.Get("page") == "1" {
					prs := make([]pullRequest, 100)
					for i := range prs {
						prs[i] = testOpenBackport(1234+i, "release/1.2.x")
					}
					return testJSON(t, prs), nil, true
				}
				if q.Get("page") != "2" {
					t.Fatalf("unexpected page: %q", path)
				}
				if match {
					return testJSON(t, []pullRequest{testOpenBackport(123, "release/1.2.x")}), nil, true
				}
				return []byte("[]"), nil, true
			}
			got, err := f.b.existing(123, "release/1.2.x")
			if err != nil || got != match || f.openReads != 2 {
				t.Fatalf("existing = %v, %v; pages=%d, want match=%v and 2 pages", got, err, f.openReads, match)
			}
		})
	}
	t.Run("page two fails closed", func(t *testing.T) {
		f := newFakeBackporter(t)
		apiErr := errors.New("rate limit on page two")
		f.getHook = func(path, accept string) ([]byte, error, bool) {
			u, err := url.Parse(path)
			if err != nil {
				t.Fatal(err)
			}
			if u.Query().Get("page") == "1" {
				return testJSON(t, make([]pullRequest, 100)), nil, true
			}
			return nil, apiErr, true
		}
		if got, err := f.b.existing(123, "release/1.2.x"); got || !errors.Is(err, apiErr) {
			t.Fatalf("pagination error lost: %v, %v", got, err)
		}
	})
}

func TestRunExistingBackportSkipsGit(t *testing.T) {
	f := newFakeBackporter(t, "backport/1.2")
	f.open = []pullRequest{testOpenBackport(123, "release/1.2.x")}
	if err := f.b.run(f.event("closed", "")); err != nil {
		t.Fatal(err)
	}
	if len(f.created)+len(f.fetched)+len(f.completed)+len(f.probed) != 0 {
		t.Fatalf("existing PR should bypass creator and all Git: create=%v fetch=%v complete=%v present=%v", f.createdLabels(), f.fetched, f.completed, f.probed)
	}
}

func TestRunCopiedBackportReferenceDoesNotSuppress(t *testing.T) {
	for _, otherSource := range []int{456, 1234} {
		t.Run(fmt.Sprintf("actual_source_%d", otherSource), func(t *testing.T) {
			f := newFakeBackporter(t, "backport/1.2")
			other := testOpenBackport(otherSource, "release/1.2.x")
			other.Body += "\n\nOriginal PR body:\n" + testOpenBackport(123, "release/1.2.x").Body
			f.open = []pullRequest{other}
			if err := f.b.run(f.event("closed", "")); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(f.createdLabels(), []string{"backport/1.2"}) {
				t.Fatal("a reference copied below another source's provenance suppressed the backport")
			}
			if len(f.completed) != 1 || len(f.probed) != 1 || f.openReads != 2 {
				t.Fatalf("copied provenance bypassed guards: complete=%v present=%v open reads=%d", f.completed, f.probed, f.openReads)
			}
		})
	}
}

func TestRunInvalidSourceSHA(t *testing.T) {
	for _, sha := range []string{"", "null", "abc123", strings.Repeat("a", 39), strings.Repeat("a", 41), strings.Repeat("g", 40), strings.Repeat("A", 40), testSource + "\n"} {
		t.Run(fmt.Sprintf("%q", sha), func(t *testing.T) {
			f := newFakeBackporter(t, "backport/1.2")
			f.pr["merge_commit_sha"] = sha
			err := f.b.run(f.event("closed", ""))
			if err == nil || !strings.Contains(err.Error(), "valid merge commit") {
				t.Fatalf("invalid source SHA: %v", err)
			}
			if len(f.fetched)+len(f.created)+len(f.completed)+len(f.probed) != 0 {
				t.Fatal("invalid source reached Git or creator")
			}
		})
	}
}

func TestRunAPIFailuresNeverCreate(t *testing.T) {
	apiErr := errors.New("GitHub API returned HTTP 403/rate limit")
	for _, tc := range []struct {
		name, stage, response string
		err                   error
	}{
		{"repository forbidden", "repo", "", apiErr},
		{"repository missing", "repo", "null", nil},
		{"repository invalid", "repo", "{", nil},
		{"repository wrong", "repo", `{"full_name":"fork/consul","default_branch":"main"}`, nil},
		{"default branch missing", "repo", `{"full_name":"hashicorp/consul"}`, nil},
		{"PR rate limited", "pr", "", apiErr},
		{"PR missing", "pr", "null", nil},
		{"PR invalid", "pr", "{", nil},
		{"PR number missing", "pr", `{"base":{"ref":"main","repo":{"full_name":"hashicorp/consul"}}}`, nil},
		{"PR wrong repository", "pr", `{"number":123,"base":{"ref":"main","repo":{"full_name":"fork/consul"}}}`, nil},
		{"PR base missing", "pr", `{"number":123,"base":{"repo":{"full_name":"hashicorp/consul"}}}`, nil},
		{"manifest missing", "manifest", "", apiErr},
		{"manifest invalid", "manifest", "not valid HCL", nil},
		{"manifest schema invalid", "manifest", "schema = 2\nactive_versions {}", nil},
		{"open PR forbidden", "open", "", apiErr},
		{"open PR missing", "open", "null", nil},
		{"open PR invalid", "open", "{", nil},
		{"open PR wrong type", "open", `{}`, nil},
		{"aggregate diff missing", "diff", "", apiErr},
		{"recheck PR forbidden", "recheck-pr", "", apiErr},
		{"recheck manifest forbidden", "recheck-manifest", "", apiErr},
		{"final open PR forbidden", "recheck-open", "", apiErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t, "backport/1.2")
			f.getHook = func(path, accept string) ([]byte, error, bool) {
				stage := ""
				switch {
				case path == "repos/"+repository:
					stage = "repo"
				case strings.Contains(path, "/pulls?"):
					stage = "open"
					if tc.stage == "recheck-open" && f.openReads == 2 {
						stage = tc.stage
					}
				case strings.Contains(path, "/contents/"):
					stage = "manifest"
					if tc.stage == "recheck-manifest" && f.manifestReads == 3 {
						stage = tc.stage
					}
				case accept == "application/vnd.github.diff":
					stage = "diff"
				default:
					stage = "pr"
					if tc.stage == "recheck-pr" && f.prReads == 3 {
						stage = tc.stage
					}
				}
				return []byte(tc.response), tc.err, stage == tc.stage
			}
			err := f.b.run(f.event("closed", ""))
			if err == nil {
				t.Fatal("API failure was ignored")
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("API error not propagated: %v", err)
			}
			if len(f.created) != 0 {
				t.Fatalf("created after API failure: %v", f.createdLabels())
			}
		})
	}
}

func TestRunGitFailuresNeverCreate(t *testing.T) {
	gitErr := errors.New("local Git/probe failure")
	for _, stage := range []string{"source fetch", "source mismatch", "unsupported source", "aggregate mismatch", "target fetch", "last fetch", "probe"} {
		t.Run(stage, func(t *testing.T) {
			f := newFakeBackporter(t, "backport/1.2")
			targetFetches := 0
			f.fetchHook = func(ref string) (string, error) {
				if ref == testSource {
					switch stage {
					case "source fetch":
						return "", gitErr
					case "source mismatch":
						return testMoved, nil
					}
					return testSource, nil
				}
				targetFetches++
				if stage == "target fetch" || (stage == "last fetch" && targetFetches == 2) {
					return "", gitErr
				}
				return testTarget, nil
			}
			f.completeHook = func(string, []byte) error {
				if stage == "unsupported source" || stage == "aggregate mismatch" {
					return gitErr
				}
				return nil
			}
			f.presentHook = func(string, string) (bool, error) {
				if stage == "probe" {
					return false, gitErr
				}
				return false, nil
			}
			err := f.b.run(f.event("closed", ""))
			if err == nil || (stage != "source mismatch" && !errors.Is(err, gitErr)) {
				t.Fatalf("Git failure not propagated: %v", err)
			}
			if len(f.created) != 0 {
				t.Fatal("creator ran after Git failure")
			}
			if (stage == "source fetch" || stage == "source mismatch" || stage == "unsupported source" || stage == "aggregate mismatch") && len(f.probed) != 0 {
				t.Fatal("source must be validated before target probe")
			}
		})
	}
}

func TestRunTargetMovement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tips    []string
		probes  int
		wantErr bool
	}{
		{"moves once then stabilizes", []string{testTarget, testMoved, testMoved, testMoved}, 2, false},
		{"moves on all three attempts", []string{testTarget, testMoved, testMoved, testTarget, testTarget, testMoved}, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t, "backport/1.2")
			next := 0
			f.fetchHook = func(ref string) (string, error) {
				if ref == testSource {
					return testSource, nil
				}
				if next >= len(tc.tips) {
					t.Fatal("exceeded bounded fetch attempts")
				}
				tip := tc.tips[next]
				next++
				return tip, nil
			}
			err := f.b.run(f.event("closed", ""))
			if (err != nil) != tc.wantErr || (tc.wantErr && !strings.Contains(err.Error(), "kept moving")) {
				t.Fatalf("run error = %v, want error = %v", err, tc.wantErr)
			}
			if len(f.probed) != tc.probes || len(f.completed) != 1 || next != len(tc.tips) {
				t.Fatalf("probes=%v complete=%v target fetches=%d", f.probed, f.completed, next)
			}
			for i, call := range f.probed {
				if call != [2]string{testSource, tc.tips[2*i]} {
					t.Errorf("probe %d used stale target: %v", i, call)
				}
				t.Run("present result discarded when target moves", func(t *testing.T) {
					f := newFakeBackporter(t, "backport/1.2")
					tips := []string{testTarget, testMoved, testMoved, testMoved}
					next := 0
					f.fetchHook = func(ref string) (string, error) {
						if ref == testSource {
							return testSource, nil
						}
						if next >= len(tips) {
							t.Fatal("unexpected extra target fetch")
						}
						tip := tips[next]
						next++
						return tip, nil
					}
					f.presentHook = func(source, target string) (bool, error) { return target == testTarget, nil }
					if err := f.b.run(f.event("closed", "")); err != nil {
						t.Fatal(err)
					}
					if len(f.probed) != 2 || len(f.created) != 1 {
						t.Fatal("stale positive presence result suppressed required backport")
					}
				})
			}
			wantCreates := 1
			if tc.wantErr {
				wantCreates = 0
			}
			if len(f.created) != wantCreates {
				t.Fatalf("created %d, want %d", len(f.created), wantCreates)
			}
		})
	}
}

func TestRunRechecks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prRead  int
		change  string
		wantErr bool
	}{
		{"label removed before probing", 2, "label", false},
		{"label removed before creator", 3, "label", false},
		{"deactivated before probing", 2, "manifest", false},
		{"deactivated before creator", 3, "manifest", false},
		{"source SHA changed", 3, "source", true},
		{"source base changed", 3, "base", true},
		{"source no longer merged", 3, "merged", true},
		{"existing backport appears before creator", 3, "open", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t, "backport/1.2")
			e := f.event("closed", "")
			f.getHook = func(path, accept string) ([]byte, error, bool) {
				if f.prReads >= tc.prRead {
					switch tc.change {
					case "label":
						f.pr["labels"] = testLabels("bug")
					case "manifest":
						f.manifest = strings.ReplaceAll(testManifest, "ce_active = true", "ce_active = false")
					case "source":
						f.pr["merge_commit_sha"] = testMoved
					case "base":
						f.pr["base"].(map[string]any)["ref"] = "release/1.22.x"
					case "merged":
						f.pr["merged"] = false
					case "open":
						f.open = []pullRequest{testOpenBackport(123, "release/1.2.x")}
					}
				}
				return nil, nil, false
			}
			err := f.b.run(e)
			if (err != nil) != tc.wantErr {
				t.Fatalf("run error = %v, want error=%v", err, tc.wantErr)
			}
			if len(f.created) != 0 {
				t.Fatal("creator ignored changed request")
			}
			if tc.prRead == 2 && len(f.fetched) != 0 {
				t.Fatal("removed request should not fetch Git")
			}
		})
	}
}

func TestRunRechecksRequestsChangedDuringPresenceProbe(t *testing.T) {
	for _, tc := range []struct {
		name          string
		change        func(*fakeBackporter)
		manifestReads int
	}{
		{
			name: "label removed during probe",
			change: func(f *fakeBackporter) {
				f.pr["labels"] = testLabels("bug")
			},
			manifestReads: 2,
		},
		{
			name: "CE deactivated during probe",
			change: func(f *fakeBackporter) {
				f.manifest = strings.ReplaceAll(testManifest, "ce_active = true", "ce_active = false")
			},
			manifestReads: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t, "backport/1.2")
			f.presentHook = func(source, target string) (bool, error) {
				if f.prReads != 2 || f.manifestReads != 2 {
					t.Fatalf("probe started without fresh request validation: PR reads=%d manifest reads=%d", f.prReads, f.manifestReads)
				}
				tc.change(f)
				return false, nil
			}
			if err := f.b.run(f.event("closed", "")); err != nil {
				t.Fatal(err)
			}
			if len(f.created) != 0 {
				t.Fatal("request changed during probe but creator still ran")
			}
			wantFetches := []string{testSource, "refs/heads/release/1.2.x", "refs/heads/release/1.2.x"}
			if !slices.Equal(f.fetched, wantFetches) || len(f.completed) != 1 || len(f.probed) != 1 {
				t.Fatalf("fixture did not finish source validation and stable-target probing: fetches=%v complete=%v probes=%v", f.fetched, f.completed, f.probed)
			}
			if f.prReads != 3 || f.manifestReads != tc.manifestReads || f.openReads != 1 {
				t.Fatalf("late request recheck did not stop creation: PR reads=%d manifest reads=%d open reads=%d", f.prReads, f.manifestReads, f.openReads)
			}
		})
	}
}

func TestRunCreatorFailuresAndRetry(t *testing.T) {
	creatorErr := errors.New("creator failed after opening PR")
	f := newFakeBackporter(t, "backport/1.2")
	e := f.event("closed", "")
	f.createHook = func([]byte) error {
		f.open = []pullRequest{testOpenBackport(123, "release/1.2.x")}
		return creatorErr
	}
	if err := f.b.run(e); !errors.Is(err, creatorErr) {
		t.Fatalf("creator error not propagated: %v", err)
	}
	fetches, probes, completions := len(f.fetched), len(f.probed), len(f.completed)
	if err := f.b.run(e); err != nil {
		t.Fatal(err)
	}
	if len(f.created) != 1 || len(f.fetched) != fetches || len(f.probed) != probes || len(f.completed) != completions {
		t.Fatal("retry duplicated creator or unnecessarily repeated Git despite existing PR")
	}
	t.Run("umbrella error", func(t *testing.T) {
		f := newFakeBackporter(t, "backport/all")
		f.createHook = func([]byte) error { return creatorErr }
		err := f.b.run(f.event("closed", ""))
		if !errors.Is(err, creatorErr) || !strings.Contains(err.Error(), "expand backport/all") {
			t.Fatalf("umbrella failure not propagated: %v", err)
		}
		if !slices.Equal(f.createdLabels(), []string{"backport/all"}) || len(f.fetched) != 0 {
			t.Fatal("umbrella failure continued to ordinary creator")
		}
	})
}

func TestCreatorCommand(t *testing.T) {
	const token = "test-token-must-not-appear-in-argv"
	t.Setenv("GITHUB_TOKEN", token)
	payload := []byte(`{"pull_request":{"title":"$(touch should-never-run); ` + "`commands`" + `","labels":[{"name":"backport/all"}]}}`)
	cmd := creatorCommand(payload)
	want := []string{
		"docker", "run", "--rm", "-i",
		"--env", "GITHUB_TOKEN",
		"--env", "GITHUB_REPOSITORY=hashicorp/consul",
		"--env", "GITHUB_EVENT_NAME=pull_request_target",
		"--env", "GITHUB_EVENT_PATH=/tmp/backport-event.json",
		"--env", `BACKPORT_LABEL_REGEXP=^backport/(?P<target>\d+\.\d+)$`,
		"--env", "BACKPORT_TARGET_TEMPLATE=release/{{.target}}.x",
		"--env", "BACKPORT_MERGE_COMMIT=true",
		"--env", "ENABLE_VERSION_MANIFESTS=true",
		"--entrypoint", "sh", "hashicorpdev/backport-assistant:v0.5.8", "-ec",
		`umask 077; cat > "$GITHUB_EVENT_PATH"; exec backport-assistant backport -merge-method=squash`,
	}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("creator command differs from restricted pinned invocation:\n got: %q\nwant: %q", cmd.Args, want)
	}
	for _, arg := range cmd.Args {
		if strings.Contains(arg, token) || strings.Contains(arg, "should-never-run") {
			t.Fatalf("token or PR metadata interpolated into argv: %q", arg)
		}
	}
	got, err := io.ReadAll(cmd.Stdin)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("creator stdin = %q, %v", got, err)
	}
	if cmd.Process != nil {
		t.Fatal("creator command must only be constructed, never executed by this test")
	}
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type testResponseBody struct {
	io.Reader
	closed bool
}

func (b *testResponseBody) Close() error {
	b.closed = true
	return nil
}

type testReadError struct{}

func (testReadError) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type testInfiniteReader struct{}

func (testInfiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestRuntimeGet(t *testing.T) {
	for _, accept := range []string{"", "application/vnd.github.diff", "application/vnd.github.raw+json"} {
		t.Run("accept="+accept, func(t *testing.T) {
			body := &testResponseBody{Reader: strings.NewReader("response")}
			r := runtimeClient{
				token: "test-token",
				http: &http.Client{Transport: testTransport(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.String() != "https://api.github.com/repos/hashicorp/consul" {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					}
					wantAccept := accept
					if wantAccept == "" {
						wantAccept = "application/vnd.github+json"
					}
					if req.Header.Get("Accept") != wantAccept || req.Header.Get("Authorization") != "Bearer test-token" ||
						req.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
						t.Fatalf("incorrect request headers: %v", req.Header)
					}
					return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
				})},
			}
			got, err := r.get("repos/"+repository, accept)
			if err != nil || string(got) != "response" || !body.closed {
				t.Fatalf("get = %q, %v; body closed=%v", got, err, body.closed)
			}
		})
	}
	for _, status := range []int{403, 404, 429, 500} {
		t.Run(fmt.Sprintf("HTTP%d", status), func(t *testing.T) {
			body := &testResponseBody{Reader: strings.NewReader(`{"message":"sensitive response"}`)}
			r := runtimeClient{http: &http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: body}, nil
			})}}
			got, err := r.get("repos/"+repository+"/pulls/123", "")
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", status)) || got != nil || !body.closed {
				t.Fatalf("get = %q, %v; body closed=%v", got, err, body.closed)
			}
		})
	}
	for _, tc := range []struct {
		name, want string
		reader     io.Reader
	}{
		{"read failure", "unexpected EOF", testReadError{}},
		{"oversized response", "exceeded size limit", io.LimitReader(testInfiniteReader{}, (32<<20)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &testResponseBody{Reader: tc.reader}
			r := runtimeClient{http: &http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: body}, nil
			})}}
			got, err := r.get("repos/"+repository, "")
			if err == nil || !strings.Contains(err.Error(), tc.want) || got != nil || !body.closed {
				t.Fatalf("get = %d bytes, %v; body closed=%v", len(got), err, body.closed)
			}
		})
	}
	t.Run("transport failure", func(t *testing.T) {
		wantErr := errors.New("mock transport failed")
		r := runtimeClient{http: &http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) {
			return nil, wantErr
		})}}
		if _, err := r.get("repos/"+repository, ""); !errors.Is(err, wantErr) {
			t.Fatalf("transport error not propagated: %v", err)
		}
	})
}

type localGit struct {
	t *testing.T
	r runtimeClient
}

func newLocalGit(t *testing.T) *localGit {
	t.Helper()
	// Keep all fixtures and the shell probe's scratch files inside this package.
	dir, err := os.MkdirTemp(".", ".test-git-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	absolute, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	repoDir, scratchDir := filepath.Join(absolute, "repo"), filepath.Join(absolute, "scratch")
	for _, path := range []string{repoDir, scratchDir} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	helper, err := filepath.Abs("../backport-patch-present.sh")
	if err != nil {
		t.Fatal(err)
	}
	env := make([]string, 0, len(os.Environ())+8)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") && !strings.HasPrefix(entry, "GITHUB_") && !strings.HasPrefix(entry, "TMPDIR=") {
			env = append(env, entry)
		}
	}
	env = append(env, "TMPDIR="+scratchDir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	g := &localGit{t: t, r: runtimeClient{dir: repoDir, helper: helper, env: env}}
	g.git("init", "--quiet", "--initial-branch=base")
	g.git("config", "user.name", "Local Backport Test")
	g.git("config", "user.email", "backport-test@example.invalid")
	g.git("config", "commit.gpgsign", "false")
	g.git("config", "core.hooksPath", scratchDir)
	g.git("config", "core.filemode", "true")
	return g
}

func (g *localGit) git(args ...string) string {
	g.t.Helper()
	data, err := g.r.git(nil, args...)
	if err != nil {
		g.t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func (g *localGit) write(path, content string) {
	g.t.Helper()
	fullPath := filepath.Join(g.r.dir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
		g.t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		g.t.Fatal(err)
	}
}

func (g *localGit) remove(path string) {
	g.t.Helper()
	if err := os.Remove(filepath.Join(g.r.dir, path)); err != nil {
		g.t.Fatal(err)
	}
}

func (g *localGit) commit(message string) string {
	g.t.Helper()
	g.git("add", "--all")
	g.git("commit", "--quiet", "--allow-empty", "-m", message)
	return g.git("rev-parse", "HEAD")
}

func (g *localGit) checkout(branch, commit string) {
	g.t.Helper()
	g.git("checkout", "--quiet", "-B", branch, commit)
}

func (g *localGit) patch(base, head string, binary bool) []byte {
	g.t.Helper()
	args := []string{"diff", "--full-index", "--find-renames", "--no-ext-diff", "--no-textconv"}
	if binary {
		args = append(args, "--binary")
	}
	args = append(args, base, head)
	data, err := g.r.git(nil, args...)
	if err != nil {
		g.t.Fatal(err)
	}
	return data
}

func (g *localGit) dirtyCaller() {
	g.t.Helper()
	g.write("caller-staged", "staged contents\n")
	g.git("add", "caller-staged")
	g.write("caller-staged", "unstaged contents\n")
	g.write("caller-untracked", "do not modify\n")
}

func (g *localGit) snapshot() []string {
	g.t.Helper()
	result := []string{
		g.git("rev-parse", "HEAD"),
		g.git("symbolic-ref", "HEAD"),
		g.git("status", "--porcelain=v1", "--untracked-files=all"),
		g.git("for-each-ref", "--format=%(refname) %(objectname)"),
		g.git("ls-files", "--stage"),
		g.git("diff", "--binary"),
		g.git("diff", "--cached", "--binary"),
		g.git("remote", "--verbose"),
	}
	for _, path := range []string{"caller-staged", "caller-untracked"} {
		data, err := os.ReadFile(filepath.Join(g.r.dir, path))
		if err != nil {
			g.t.Fatal(err)
		}
		result = append(result, string(data))
	}
	return result
}

func TestRuntimeCompleteAndPresentLocalGit(t *testing.T) {
	const original = "alpha\nbefore\nomega\n"
	const changed = "alpha\nafter\nomega\n"
	const block = "one\ntwo\nthree\nfour\nbefore\nsix\nseven\neight\nnine\n"
	for _, tc := range []struct {
		name   string
		source func(*localGit)
		target func(*localGit, string, string)
		want   bool
	}{
		{
			name:   "equivalent independent commit",
			source: func(g *localGit) { g.write("file", changed) },
			target: func(g *localGit, _, _ string) { g.write("file", changed) },
			want:   true,
		},
		{
			name: "equivalent changes split over commits",
			source: func(g *localGit) {
				g.write("file", changed)
				g.write("second", changed)
			},
			target: func(g *localGit, _, _ string) {
				g.write("file", changed)
				g.commit("first half")
				g.write("second", changed)
			},
			want: true,
		},
		{
			name:   "equivalent change in a larger commit",
			source: func(g *localGit) { g.write("file", changed) },
			target: func(g *localGit, _, _ string) {
				g.write("file", changed)
				g.write("second", "unrelated larger change\n")
				g.write("extra", "more target-only content\n")
			},
			want: true,
		},
		{
			name: "partially applied changes",
			source: func(g *localGit) {
				g.write("file", changed)
				g.write("second", changed)
			},
			target: func(g *localGit, _, _ string) { g.write("file", changed) },
		},
		{
			name:   "conflicting change",
			source: func(g *localGit) { g.write("file", changed) },
			target: func(g *localGit, _, _ string) { g.write("file", "alpha\nconflict\nomega\n") },
		},
		{
			name:   "source ancestor subsequently reverted",
			source: func(g *localGit) { g.write("file", changed) },
			target: func(g *localGit, _, source string) {
				g.checkout("target", source)
				g.git("revert", "--no-edit", source)
			},
		},
		{
			name:   "cherry-picked change subsequently reverted",
			source: func(g *localGit) { g.write("file", changed) },
			target: func(g *localGit, _, source string) {
				g.git("cherry-pick", "-x", source)
				picked := g.git("rev-parse", "HEAD")
				if picked == source {
					t.Fatal("test requires a distinct cherry-picked commit")
				}
				g.git("revert", "--no-edit", picked)
			},
		},
		{
			name: "repeated block reverse applies to wrong hunk",
			source: func(g *localGit) {
				g.write("file", strings.Replace(block, "before", "after", 1)+"separator\n"+block)
			},
			target: func(g *localGit, _, source string) {
				g.write("file", block+"separator\n"+strings.Replace(block, "before", "after", 1))
				g.commit("change the wrong repeated block")
				if _, err := g.r.git(g.patch(source+"^", source, true), "apply", "--reverse", "--check"); err != nil {
					t.Fatalf("fixture must demonstrate that reverse-apply alone is unsound: %v", err)
				}
			},
		},
		{
			name:   "binary addition",
			source: func(g *localGit) { g.write("binary", "\x00\x01\x02new\xff") },
			target: func(g *localGit, _, _ string) { g.write("binary", "\x00\x01\x02new\xff") },
			want:   true,
		},
		{
			name:   "binary content differs",
			source: func(g *localGit) { g.write("binary", "\x00\x01\x02new\xff") },
			target: func(g *localGit, _, _ string) { g.write("binary", "\x00\x01\x02different\xff") },
		},
		{
			name:   "binary modification",
			source: func(g *localGit) { g.write("binary-existing", "\x00\x01\x02new\xff") },
			target: func(g *localGit, _, _ string) { g.write("binary-existing", "\x00\x01\x02new\xff") },
			want:   true,
		},
		{
			name:   "binary deletion",
			source: func(g *localGit) { g.remove("binary-existing") },
			target: func(g *localGit, _, _ string) { g.remove("binary-existing") },
			want:   true,
		},
		{
			name:   "file deletion",
			source: func(g *localGit) { g.remove("second") },
			target: func(g *localGit, _, _ string) { g.remove("second") },
			want:   true,
		},
		{
			name:   "file addition",
			source: func(g *localGit) { g.write("new", "added content\n") },
			target: func(g *localGit, _, _ string) { g.write("new", "added content\n") },
			want:   true,
		},
		{
			name:   "different added file",
			source: func(g *localGit) { g.write("new", "added content\n") },
			target: func(g *localGit, _, _ string) { g.write("new", "other content\n") },
		},
		{
			name:   "empty file addition",
			source: func(g *localGit) { g.write("new", "") },
			target: func(g *localGit, _, _ string) { g.write("new", "") },
			want:   true,
		},
		{
			name:   "rename",
			source: func(g *localGit) { g.git("mv", "file", "renamed") },
			target: func(g *localGit, _, _ string) { g.git("mv", "file", "renamed") },
			want:   true,
		},
		{
			name: "executable filemode change",
			source: func(g *localGit) {
				if err := os.Chmod(filepath.Join(g.r.dir, "file"), 0755); err != nil {
					g.t.Fatal(err)
				}
			},
			target: func(g *localGit, _, _ string) {
				if err := os.Chmod(filepath.Join(g.r.dir, "file"), 0755); err != nil {
					g.t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "symlink addition",
			source: func(g *localGit) {
				if err := os.Symlink("file", filepath.Join(g.r.dir, "link")); err != nil {
					g.t.Fatal(err)
				}
			},
			target: func(g *localGit, _, _ string) {
				if err := os.Symlink("file", filepath.Join(g.r.dir, "link")); err != nil {
					g.t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "symlink points elsewhere",
			source: func(g *localGit) {
				if err := os.Symlink("file", filepath.Join(g.r.dir, "link")); err != nil {
					g.t.Fatal(err)
				}
			},
			target: func(g *localGit, _, _ string) {
				if err := os.Symlink("second", filepath.Join(g.r.dir, "link")); err != nil {
					g.t.Fatal(err)
				}
			},
		},
		{
			name:   "empty patch",
			source: func(*localGit) {},
			target: func(*localGit, string, string) {},
			want:   true,
		},
		{
			name:   "source is an unchanged ancestor",
			source: func(g *localGit) { g.write("file", changed) },
			target: func(g *localGit, _, source string) {
				g.checkout("target", source)
				g.write("extra", "unrelated later commit\n")
			},
			want: true,
		},
		{
			name:   "patch not applied",
			source: func(g *localGit) { g.write("file", changed) },
			target: func(*localGit, string, string) {},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newLocalGit(t)
			content := original
			if tc.name == "repeated block reverse applies to wrong hunk" {
				content = block + "separator\n" + block
			}
			g.write("file", content)
			g.write("second", original)
			g.write("binary-existing", "\x00original binary\n")
			base := g.commit("base")
			g.checkout("source", base)
			tc.source(g)
			source := g.commit("source aggregate change")
			g.checkout("target", base)
			tc.target(g, base, source)
			target := g.commit("independent target")
			g.dirtyCaller()
			before := g.snapshot()
			defer func() {
				if after := g.snapshot(); !slices.Equal(after, before) {
					t.Errorf("probe changed caller HEAD, index, worktree, refs, or remotes:\n before %q\n after %q", before, after)
				}
			}()
			if err := g.r.complete(source, g.patch(source+"^", source, false)); err != nil {
				t.Fatalf("complete source rejected: %v", err)
			}
			present, err := g.r.present(source, target)
			if err != nil || present != tc.want {
				t.Fatalf("present = %v, %v; want %v", present, err, tc.want)
			}
		})
	}
}

func TestRuntimeCompleteRejectsMissingOrIncompleteAggregate(t *testing.T) {
	g := newLocalGit(t)
	g.write("file", "initial\n")
	base := g.commit("base")
	g.write("file", "initial\nfirst change\n")
	first := g.commit("first rebased PR commit")
	g.write("second", "second change\n")
	final := g.commit("final rebased PR commit")
	empty := g.commit("empty")
	firstPatch := g.patch(base, first, false)
	finalPatch := g.patch(first, final, false)
	aggregate := g.patch(base, final, false)
	g.dirtyCaller()
	before := g.snapshot()
	for _, tc := range []struct {
		name, source, wantError string
		diff                    []byte
	}{
		{"complete single commit", first, "", firstPatch},
		{"complete final commit alone", final, "", finalPatch},
		{"multi-commit rebase final-only source", final, "complete PR diff", aggregate},
		{"first-only source", first, "complete PR diff", aggregate},
		{"nonempty source missing aggregate", first, "complete PR diff", nil},
		{"nonempty source whitespace aggregate", first, "complete PR diff", []byte(" \n\t")},
		{"empty source empty aggregate", empty, "", nil},
		{"empty source whitespace aggregate", empty, "", []byte(" \n\t")},
		{"empty source nonempty aggregate", empty, "complete PR diff", firstPatch},
		{"HTML instead of API diff", first, "aggregate PR diff", []byte("<html>rate limit</html>")},
		{"JSON instead of API diff", first, "aggregate PR diff", []byte(`{"message":"not found"}`)},
		{"truncated diff header", first, "complete PR diff", []byte("diff --git a/file b/file\n")},
		{"changed whitespace is significant", first, "complete PR diff", bytes.ReplaceAll(firstPatch, []byte("+first change"), []byte("+first  change"))},
		{"missing source object", testSource, "git show", firstPatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := g.r.complete(tc.source, tc.diff)
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("complete error = %v, want %q", err, tc.wantError)
			}
			if after := g.snapshot(); !slices.Equal(before, after) {
				t.Fatal("completeness check changed caller state")
			}
		})
	}
}

func TestRuntimeRejectsRootAndMergeSources(t *testing.T) {
	g := newLocalGit(t)
	g.write("file", "base\n")
	root := g.commit("root")
	g.checkout("source", root)
	g.write("file", "source\n")
	source := g.commit("source")
	g.checkout("target", root)
	g.write("second", "target\n")
	target := g.commit("target")
	g.git("merge", "--no-ff", "-m", "merge source", source)
	merge := g.git("rev-parse", "HEAD")
	g.dirtyCaller()
	before := g.snapshot()
	for _, tc := range []struct{ name, source string }{{"root", root}, {"merge", merge}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := g.r.complete(tc.source, nil); err == nil || !strings.Contains(err.Error(), "root/merge") {
				t.Fatalf("unsupported source complete error = %v", err)
			}
			if present, err := g.r.present(tc.source, target); err == nil || present {
				t.Fatalf("unsupported source present = %v, %v", present, err)
			}
			if after := g.snapshot(); !slices.Equal(before, after) {
				t.Fatal("unsupported source probe changed caller")
			}
		})
	}
}

func TestRunRuntimeCompletenessFailuresNeverCreate(t *testing.T) {
	g := newLocalGit(t)
	g.write("file", "initial\n")
	root := g.commit("root")
	g.write("file", "first PR change\n")
	first := g.commit("first rebased PR commit")
	g.write("second", "second PR change\n")
	final := g.commit("final rebased PR commit")
	for _, tc := range []struct {
		name, source string
		diff         []byte
	}{
		{"unsupported root source", root, nil},
		{"multi-commit rebase incomplete source", final, g.patch(root, final, false)},
		{"source diff does not match PR", final, g.patch(root, first, false)},
		{"API empty aggregate", final, nil},
		{"API invalid aggregate", final, []byte("<html>error</html>")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackporter(t, "backport/1.2")
			f.pr["merge_commit_sha"] = tc.source
			f.diff = tc.diff
			f.completeHook = g.r.complete
			if err := f.b.run(f.event("closed", "")); err == nil {
				t.Fatal("real Git source validation unexpectedly succeeded")
			}
			if len(f.created) != 0 || len(f.probed) != 0 || len(f.fetched) != 1 || len(f.completed) != 1 {
				t.Fatalf("unsupported/mismatched source escaped completeness guard: created=%v probes=%v fetches=%v", f.createdLabels(), f.probed, f.fetched)
			}
		})
	}
}

func TestRuntimeGitAndProbeErrors(t *testing.T) {
	g := newLocalGit(t)
	g.write("file", "base\n")
	base := g.commit("base")
	g.write("file", "change\n")
	source := g.commit("source")
	if _, err := g.r.git(nil, "rev-parse", "--verify", "does-not-exist"); err == nil || !strings.Contains(err.Error(), "git rev-parse") {
		t.Fatalf("Git error lost context: %v", err)
	}
	for _, tc := range []struct{ name, source, target string }{
		{"missing target", source, testTarget},
		{"missing source", testSource, base},
	} {
		t.Run(tc.name, func(t *testing.T) {
			present, err := g.r.present(tc.source, tc.target)
			if err == nil || present || !strings.Contains(err.Error(), "cannot establish patch presence") {
				t.Fatalf("probe error was treated as presence: %v, %v", present, err)
			}
		})
	}
	g.r.helper = filepath.Join(g.r.dir, "missing-helper.sh")
	if present, err := g.r.present(source, base); err == nil || present {
		t.Fatalf("missing helper = %v, %v", present, err)
	}
}

func TestRuntimeFetchLocalRepositoryRetainsTips(t *testing.T) {
	origin := newLocalGit(t)
	origin.write("file", "base\n")
	first := origin.commit("first")
	client := newLocalGit(t)
	// Git's URL rewrite ensures runtimeClient.fetch never accesses GitHub.
	client.git("config", "url."+origin.r.dir+"/.insteadOf", "https://github.com/"+repository+".git")
	client.git("config", "protocol.file.allow", "always")
	for i := 0; i < 2; i++ {
		want := first
		if i == 1 {
			origin.write("file", "advanced\n")
			want = origin.commit("second")
		}
		got, err := client.r.fetch("refs/heads/base")
		if err != nil || got != want {
			t.Fatalf("fetch = %q, %v; want %q", got, err, want)
		}
		if got := client.git("rev-parse", "refs/backport-cache/"+want); got != want {
			t.Fatalf("fetched tip not retained for negotiation: %q", got)
		}
		if got := client.git("rev-parse", "refs/backport-cache/"+first); got != first {
			t.Fatal("previous fetched tip lost")
		}
	}
	if got, err := client.r.fetch(first); err != nil || got != first {
		t.Fatalf("fetch immutable source = %q, %v", got, err)
	}
	if _, err := client.r.fetch("refs/heads/missing"); err == nil {
		t.Fatal("missing local ref should fail, never reuse stale FETCH_HEAD")
	}
}
