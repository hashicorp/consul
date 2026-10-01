// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

// ce-backport gates the existing backport-assistant creator with current-content
// and open-PR checks. It never changes labels on a PR except via the existing
// backport/all expansion in backport-assistant.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

const (
	repository = "hashicorp/consul"
	image      = "hashicorpdev/backport-assistant:v0.5.8"
)

var (
	labelPattern   = regexp.MustCompile(`^backport/([0-9]+\.[0-9]+)$`)
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)
	shaPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type label struct {
	Name string `json:"name"`
}

type repoInfo struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

type pullRequest struct {
	Number         int     `json:"number"`
	Merged         bool    `json:"merged"`
	MergeCommitSHA string  `json:"merge_commit_sha"`
	Body           string  `json:"body"`
	Labels         []label `json:"labels"`
	Base           struct {
		Ref  string   `json:"ref"`
		Repo repoInfo `json:"repo"`
	} `json:"base"`
	Head struct {
		Ref  string   `json:"ref"`
		Repo repoInfo `json:"repo"`
	} `json:"head"`
}

type event struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	Repository  repoInfo    `json:"repository"`
	PullRequest pullRequest `json:"pull_request"`
	Label       label       `json:"label"`
	raw         map[string]json.RawMessage
}

func parseEvent(data []byte) (event, error) {
	var e event
	if err := json.Unmarshal(data, &e); err != nil {
		return e, err
	}
	if err := json.Unmarshal(data, &e.raw); err != nil {
		return e, err
	}
	if e.Repository.FullName != repository || e.Number <= 0 || e.Number != e.PullRequest.Number {
		return e, errors.New("expected a hashicorp/consul pull request event")
	}
	return e, nil
}

type releaseLine struct {
	Version  string `hcl:"version,label"`
	CEActive bool   `hcl:"ce_active,optional"`
	LTS      bool   `hcl:"lts,optional"`
}

type manifest struct {
	Schema     int    `hcl:"schema,optional"`
	TargetRepo string `hcl:"target_repository,optional"`
	Active     struct {
		Versions []releaseLine `hcl:"version,block"`
	} `hcl:"active_versions,block"`
}

func readManifest(data []byte) (manifest, error) {
	var m manifest
	if err := hclsimple.Decode("versions.hcl", data, nil, &m); err != nil {
		return m, err
	}
	if m.Schema != 1 {
		return m, fmt.Errorf("unsupported versions manifest schema %d", m.Schema)
	}
	seen := make(map[string]bool)
	for _, line := range m.Active.Versions {
		if !versionPattern.MatchString(line.Version) || seen[line.Version] {
			return m, fmt.Errorf("invalid or duplicate manifest version %q", line.Version)
		}
		seen[line.Version] = true
	}
	return m, nil
}

func (m manifest) active(version string) bool {
	// Match the creator's exact-version precedence, then its release-line
	// prefix matching. LTS alone never makes a version active for CE.
	for _, line := range m.Active.Versions {
		if line.Version == version {
			return line.CEActive
		}
	}
	for _, line := range m.Active.Versions {
		if strings.HasPrefix(line.Version, version+".") && line.CEActive {
			return true
		}
	}
	return false
}

func hasLabel(pr pullRequest, name string) bool {
	return slices.ContainsFunc(pr.Labels, func(l label) bool { return l.Name == name })
}

func targets(pr pullRequest, m manifest) []string {
	var result []string
	for _, l := range pr.Labels {
		match := labelPattern.FindStringSubmatch(l.Name)
		if match != nil && m.active(match[1]) && !slices.Contains(result, match[1]) {
			result = append(result, match[1])
		}
	}
	sort.Strings(result)
	return result
}

func needsUmbrellaLabels(pr pullRequest, m manifest) bool {
	if !hasLabel(pr, "backport/all") {
		return false
	}
	for _, line := range m.Active.Versions {
		prefix := "backport/"
		if !line.CEActive {
			prefix += "ent/"
		}
		if !hasLabel(pr, prefix+line.Version) {
			return true
		}
	}
	return false
}

type backporter struct {
	get      func(path, accept string) ([]byte, error)
	fetch    func(ref string) (string, error)
	complete func(source string, diff []byte) error
	present  func(source, target string) (bool, error)
	create   func(payload []byte) error
	log      io.Writer
}

func (b *backporter) currentPR(number int) (pullRequest, []byte, error) {
	data, err := b.get(fmt.Sprintf("repos/%s/pulls/%d", repository, number), "")
	if err != nil {
		return pullRequest{}, nil, err
	}
	var pr pullRequest
	if err := json.Unmarshal(data, &pr); err != nil {
		return pr, nil, err
	}
	if pr.Number != number || pr.Base.Repo.FullName != repository || pr.Base.Ref == "" {
		return pr, nil, errors.New("invalid source PR response")
	}
	return pr, data, nil
}

func (b *backporter) loadManifest(defaultBranch string) (manifest, error) {
	// Resolve the file from the current remote default branch, never the PR's
	// checkout or its original merge revision.
	data, err := b.get("repos/"+repository+"/contents/.release/versions.hcl?ref="+url.QueryEscape(defaultBranch),
		"application/vnd.github.raw+json")
	if err != nil {
		return manifest{}, err
	}
	return readManifest(data)
}

func (b *backporter) existing(number int, branch string) (bool, error) {
	for page := 1; ; page++ {
		query := url.Values{
			"state": {"open"}, "base": {branch}, "per_page": {"100"}, "page": {fmt.Sprint(page)},
		}
		data, err := b.get("repos/"+repository+"/pulls?"+query.Encode(), "")
		if err != nil {
			return false, err
		}
		var prs []pullRequest
		if err := json.Unmarshal(data, &prs); err != nil || prs == nil {
			return false, errors.New("invalid open backport PR response")
		}
		for _, pr := range prs {
			// Match the pinned creator's provenance line, not a title, a bare
			// number search, or a similarly named branch in someone else's fork.
			if pr.Base.Ref != branch || pr.Head.Repo.FullName != repository ||
				!strings.HasPrefix(pr.Head.Ref, "backport/") {
				continue
			}
			reference := fmt.Sprintf("## Backport\n\nThis PR is auto-generated from #%d to be assessed for backporting", number)
			if strings.HasPrefix(strings.TrimSpace(pr.Body), reference+" due to the inclusion of the label ") {
				fmt.Fprintf(b.log, "Skipping %s#%d -> %s: open backport PR %d already exists.\n",
					repository, number, branch, pr.Number)
				return true, nil
			}
		}
		if len(prs) < 100 {
			return false, nil
		}
	}
}

func payloadFor(e event, prJSON []byte, selectedLabel string) ([]byte, error) {
	// Retain sender/reviewer/author metadata and original PR labels for the
	// existing creator. A labeled event restricts it to exactly one target.
	copy := make(map[string]json.RawMessage, len(e.raw))
	for k, v := range e.raw {
		copy[k] = v
	}
	copy["action"] = json.RawMessage(`"labeled"`)
	copy["pull_request"] = prJSON
	data, err := json.Marshal(label{Name: selectedLabel})
	if err != nil {
		return nil, err
	}
	copy["label"] = data
	return json.Marshal(copy)
}

func (b *backporter) requested(number int, source, defaultBranch, version string) ([]byte, bool, error) {
	pr, data, err := b.currentPR(number)
	if err != nil {
		return nil, false, err
	}
	if !pr.Merged || pr.Base.Ref != defaultBranch || pr.MergeCommitSHA != source {
		return nil, false, errors.New("source PR changed during preflight; retry the workflow")
	}
	if !hasLabel(pr, "backport/"+version) {
		fmt.Fprintf(b.log, "Skipping release/%s.x: label was removed.\n", version)
		return nil, false, nil
	}
	m, err := b.loadManifest(defaultBranch)
	if err != nil {
		return nil, false, err
	}
	if !m.active(version) {
		fmt.Fprintf(b.log, "Skipping release/%s.x: version is no longer CE-active.\n", version)
		return nil, false, nil
	}
	return data, true, nil
}

func (b *backporter) run(e event) error {
	if !e.PullRequest.Merged || (e.Action != "closed" && e.Action != "labeled") {
		fmt.Fprintln(b.log, "No merged backport event to process.")
		return nil
	}
	data, err := b.get("repos/"+repository, "")
	if err != nil {
		return err
	}
	var repo repoInfo
	if err := json.Unmarshal(data, &repo); err != nil || repo.FullName != repository || repo.DefaultBranch == "" {
		return errors.New("invalid repository response")
	}
	pr, prJSON, err := b.currentPR(e.Number)
	if err != nil {
		return err
	}
	if !pr.Merged || pr.Base.Ref != repo.DefaultBranch {
		fmt.Fprintln(b.log, "Only merged PRs into the current default branch are backported.")
		return nil
	}
	if !slices.ContainsFunc(pr.Labels, func(l label) bool {
		return l.Name == "backport/all" || labelPattern.MatchString(l.Name)
	}) {
		fmt.Fprintln(b.log, "No requested CE release backports.")
		return nil
	}
	m, err := b.loadManifest(repo.DefaultBranch)
	if err != nil {
		return err
	}
	e.Repository = repo
	e.raw["repository"] = data
	// Preserve backport/all's CE + Enterprise label expansion, but force a
	// labeled umbrella event. v0.5.8 then only adds labels, never creates PRs
	// without the per-target preflight below.
	if needsUmbrellaLabels(pr, m) {
		payload, err := payloadFor(e, prJSON, "backport/all")
		if err != nil {
			return err
		}
		if err := b.create(payload); err != nil {
			return fmt.Errorf("expand backport/all: %w", err)
		}
		pr, prJSON, err = b.currentPR(e.Number)
		if err != nil {
			return err
		}
	}

	// Reconcile all currently requested active targets. Per-PR concurrency can
	// coalesce label events; processing only the newest label would lose work.
	selected := targets(pr, m)
	if len(selected) == 0 {
		fmt.Fprintln(b.log, "No requested CE-active release targets.")
		return nil
	}
	if !shaPattern.MatchString(pr.MergeCommitSHA) {
		return errors.New("source PR has no valid merge commit; retry when GitHub has populated it")
	}
	source := pr.MergeCommitSHA
	sourceValidated := false
	for _, version := range selected {
		branch := "release/" + version + ".x"
		finished := false
		for attempt := 0; attempt < 3; attempt++ {
			currentJSON, requested, err := b.requested(pr.Number, source, repo.DefaultBranch, version)
			if err != nil {
				return err
			}
			if !requested {
				finished = true
				break
			}
			exists, err := b.existing(pr.Number, branch)
			if err != nil {
				return err
			}
			if exists {
				finished = true
				break
			}
			if !sourceValidated {
				resolved, err := b.fetch(source)
				if err != nil {
					return err
				}
				if resolved != source {
					return errors.New("fetched source does not match the merge commit")
				}
				diff, err := b.get(fmt.Sprintf("repos/%s/pulls/%d", repository, pr.Number), "application/vnd.github.diff")
				if err != nil {
					return err
				}
				if err := b.complete(source, diff); err != nil {
					return err
				}
				sourceValidated = true
			}
			tip, err := b.fetch("refs/heads/" + branch)
			if err != nil {
				return err
			}
			present, err := b.present(source, tip)
			if err != nil {
				return err
			}
			latest, err := b.fetch("refs/heads/" + branch)
			if err != nil {
				return err
			}
			if latest != tip {
				fmt.Fprintf(b.log, "%s moved during preflight; retrying.\n", branch)
				continue
			}
			if present {
				fmt.Fprintf(b.log, "Skipping %s#%d -> %s: complete change already present at %s.\n",
					repository, pr.Number, branch, tip)
				finished = true
				break
			}
			currentJSON, requested, err = b.requested(pr.Number, source, repo.DefaultBranch, version)
			if err != nil {
				return err
			}
			if !requested {
				finished = true
				break
			}
			exists, err = b.existing(pr.Number, branch)
			if err != nil {
				return err
			}
			if !exists {
				payload, err := payloadFor(e, currentJSON, "backport/"+version)
				if err != nil {
					return err
				}
				fmt.Fprintf(b.log, "Backporting %s#%d into %s using backport-assistant.\n", repository, pr.Number, branch)
				if err := b.create(payload); err != nil {
					return err
				}
			}
			finished = true
			break
		}
		if !finished {
			return fmt.Errorf("%s kept moving; rerun instead of creating a stale backport", branch)
		}
	}
	return nil
}

type runtimeClient struct {
	dir, helper string
	env         []string
	http        *http.Client
	token       string
}

func (r *runtimeClient) get(path, accept string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/"+path, nil)
	if err != nil {
		return nil, err
	}
	if accept == "" {
		accept = "application/vnd.github+json"
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub GET %s returned HTTP %d", path, resp.StatusCode)
	}
	const limit = 32 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errors.New("GitHub response exceeded size limit; refusing partial data")
	}
	return data, nil
}

func (r *runtimeClient) git(input []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env, cmd.Stdin = r.dir, r.env, bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, stderr.String())
	}
	return data, nil
}

func (r *runtimeClient) fetch(ref string) (string, error) {
	if _, err := r.git(nil, "fetch", "--quiet", "--no-tags", "https://github.com/"+repository+".git", ref); err != nil {
		return "", err
	}
	out, err := r.git(nil, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return "", err
	}
	tip := strings.TrimSpace(string(out))
	if !shaPattern.MatchString(tip) {
		return "", errors.New("fetch did not resolve a commit")
	}
	// Retain tips only in the temporary repository so subsequent fetches can
	// negotiate existing history rather than repeatedly transferring it.
	if _, err := r.git(nil, "update-ref", "refs/backport-cache/"+tip, tip); err != nil {
		return "", err
	}
	return tip, nil
}

func (r *runtimeClient) complete(source string, diff []byte) error {
	parents, err := r.git(nil, "show", "-s", "--format=%P", source)
	if err != nil {
		return err
	}
	if len(strings.Fields(string(parents))) != 1 {
		return errors.New("root/merge source commits need a manual backport")
	}
	patch, err := r.git(nil, "diff", "--binary", "--full-index", "--find-renames", "--no-ext-diff", "--no-textconv", source+"^", source)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(diff)) > 0 && !bytes.HasPrefix(diff, []byte("diff --git ")) {
		return errors.New("GitHub did not return the aggregate PR diff")
	}
	if len(patch) == 0 && len(bytes.TrimSpace(diff)) == 0 {
		return nil
	}
	commitID, err := r.git(patch, "patch-id", "--verbatim")
	if err != nil {
		return err
	}
	prID, err := r.git(diff, "patch-id", "--verbatim")
	if err != nil {
		return err
	}
	a, b := strings.Fields(string(commitID)), strings.Fields(string(prID))
	if len(a) != 2 || len(b) != 2 || a[0] != b[0] {
		return errors.New("merge commit does not cover the complete PR diff; manual backport required (for example, a multi-commit rebase merge)")
	}
	return nil
}

func (r *runtimeClient) present(source, target string) (bool, error) {
	cmd := exec.Command("sh", r.helper, r.dir, source, target)
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("cannot establish patch presence: %w: %s", err, out)
}

func creatorCommand(payload []byte) *exec.Cmd {
	// stdin avoids writable bind mounts and shell interpolation of PR metadata.
	cmd := exec.Command("docker", "run", "--rm", "-i",
		"--env", "GITHUB_TOKEN",
		"--env", "GITHUB_REPOSITORY="+repository,
		"--env", "GITHUB_EVENT_NAME=pull_request_target",
		"--env", "GITHUB_EVENT_PATH=/tmp/backport-event.json",
		"--env", `BACKPORT_LABEL_REGEXP=^backport/(?P<target>\d+\.\d+)$`,
		"--env", "BACKPORT_TARGET_TEMPLATE=release/{{.target}}.x",
		"--env", "BACKPORT_MERGE_COMMIT=true",
		"--env", "ENABLE_VERSION_MANIFESTS=true",
		"--entrypoint", "sh", image, "-ec",
		`umask 077; cat > "$GITHUB_EVENT_PATH"; exec backport-assistant backport -merge-method=squash`)
	cmd.Stdin = bytes.NewReader(payload)
	return cmd
}

func run() (returnErr error) {
	eventPath := flag.String("event", os.Getenv("GITHUB_EVENT_PATH"), "GitHub pull request event JSON")
	helper := flag.String("helper", ".github/scripts/backport-patch-present.sh", "read-only patch probe")
	flag.Parse()
	if *eventPath == "" || os.Getenv("GITHUB_REPOSITORY") != repository {
		return errors.New("expected GITHUB_EVENT_PATH and GITHUB_REPOSITORY=hashicorp/consul")
	}
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return errors.New("GITHUB_TOKEN is required")
	}
	data, err := os.ReadFile(*eventPath)
	if err != nil {
		return err
	}
	e, err := parseEvent(data)
	if err != nil {
		return err
	}
	helperPath, err := filepath.Abs(*helper)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "consul-ce-backport-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary repository: %w", err))
		}
	}()
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
		"GIT_CONFIG_VALUE_0=AUTHORIZATION: basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token)))
	r := runtimeClient{dir: dir, helper: helperPath, env: env, token: token,
		http: &http.Client{Timeout: time.Minute}}
	if _, err := r.git(nil, "init", "--quiet"); err != nil {
		return err
	}
	b := backporter{get: r.get, fetch: r.fetch, complete: r.complete, present: r.present, log: os.Stdout,
		create: func(payload []byte) error {
			cmd := creatorCommand(payload)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("backport-assistant failed: %w", err)
			}
			return nil
		}}
	return b.run(e)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
