# CI Dashboard

A standalone, read-only dashboard that shows GitHub Actions workflow results
for the Consul repository. It is built as a static site and deployed to
GitHub Pages — completely separate from the Consul binary build.

---

## Architecture overview

```
GitHub Actions API
        │
        ▼
scripts/aggregate.js          ← fetches run + job data, writes JSON
        │
        ▼
public/data/rc_aggregated_results.json
        │
        ▼
Vite build  (src/)            ← pure JS + CSS, no framework
        │
        ▼
dist/                         ← static HTML/JS/CSS bundle
        │
        ▼
GitHub Pages                  ← publicly accessible, no binary involved
```

All four stages happen inside a single GitHub Actions workflow
(`.github/workflows/aggregate-and-deploy-dashboard.yml`).  No external
services, no secrets beyond the built-in `GITHUB_TOKEN`, no dependency on
the main Consul binary build.

---

## Local development

### Prerequisites

| Tool | Version |
|------|---------|
| Node | ≥ 18 (built-in `fetch` required) |
| pnpm | ≥ 10 |
| GitHub personal access token | `repo` or `public_repo` read scope |

### Steps

```sh
# 1. Install dashboard dependencies (from the monorepo root or this folder)
cd ui/packages/ci-dashboard
pnpm install

# 2. Export your GitHub token
export GITHUB_TOKEN=ghp_yourPersonalAccessToken

# 3. Fetch workflow data for a branch and write the JSON
node scripts/aggregate.js \
  --repo    hashicorp/consul \
  --branch  main \
  --label   "nightly-main" \
  --output  public/data/rc_aggregated_results.json

# 4. Start the dev server
pnpm dev
# → http://localhost:5173
```

The `pnpm dev` server hot-reloads on source changes.
The JSON file is served as a static asset from `public/data/` — re-run
`aggregate.js` any time to pull fresher data.

### Fetching multiple branches at once

Each call to `aggregate.js` with `--existing` appends or updates a single
entry in the output array.  Run it once per branch; the dashboard's run
selector will show all of them automatically.

```sh
OUTPUT="public/data/rc_aggregated_results.json"

for branch in main release/1.20.x v2.0.0-rc3; do
  node scripts/aggregate.js \
    --repo     hashicorp/consul \
    --branch   "$branch" \
    --output   "$OUTPUT" \
    --existing "$OUTPUT"
done

pnpm dev
```

### Script flags reference

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--repo` | Yes | — | `owner/repo`, e.g. `hashicorp/consul` |
| `--branch` | Yes | — | Branch name or tag |
| `--label` | No | branch name | Human-readable run label shown in the dashboard |
| `--release-branch` | No | branch name | High-level branch label (e.g. `v2.0` for `v2.0.0-rc1`) |
| `--release-version` | No | branch name | Version string shown in the run selector |
| `--output` | No | `public/data/rc_aggregated_results.json` | Output path |
| `--existing` | No | _(empty)_ | Path to existing JSON to append/update rather than overwrite |
| `--max-runs` | No | `50` | Max workflow runs to fetch per branch (1–100) |
| `--skip-annotations` | No | `false` | Skip check-run annotation fetching (faster, no test-level failure detail) |

---

## Production deployment

### How it works

One workflow file handles the full pipeline end-to-end:

```
.github/workflows/aggregate-and-deploy-dashboard.yml
```

```
Trigger → checkout → pnpm install → aggregate.js (per branch) → vite build → GitHub Pages
```

No second workflow is needed.  The earlier `deploy-ci-dashboard.yml` was
removed because both files shared the same `concurrency: group: pages` key
and would have deadlocked each other.

### Trigger modes

| Trigger | When it fires | Branches aggregated |
|---------|--------------|---------------------|
| `workflow_dispatch` | Manually from the Actions UI | Whatever you type in the input |
| `schedule` (nightly 06:00 UTC) | Automatically every night | `main` |
| `push` to `main` (source change) | When dashboard source files change | `main` |

### Manual dispatch inputs

Navigate to **Actions → Aggregate CI Data & Deploy Dashboard → Run workflow**:

| Input | Example | Notes |
|-------|---------|-------|
| `branches` | `main,release/1.20.x,v2.0.0-rc3` | Comma-separated. Each branch becomes one entry in the run selector. |
| `release_version_prefix` | `nightly` | Produces labels like `nightly-main`. Leave blank to use the branch name directly. |
| `max_runs` | `30` | Higher = more history fetched but slower. Max 100. |

### Permissions

The workflow uses **only** the built-in `secrets.GITHUB_TOKEN`.  No PAT or
extra secrets are required.  The token is granted these exact permissions:

```yaml
permissions:
  contents: read    # checkout the source
  pages: write      # deploy to GitHub Pages
  id-token: write   # OIDC token for Pages authentication
  actions: read     # read workflow run + job data via the API
```

The token is never written into any build artifact or the deployed JSON.

---

## Data flow and security

### What the JSON contains

The `rc_aggregated_results.json` file (and thus the deployed dashboard) contains:

- Job names and workflow names
- Statuses (`passed` / `failed` / `skipped`)
- Pass rates and counts
- Commit SHAs (first 12 characters shown in the UI, full SHA in the file)
- Timestamps

It does **not** contain:

- The `GITHUB_TOKEN` or any other secret
- Internal hostnames, IPs, or environment variables from runner environments
- Log output or stdout from jobs
- User data, PII, or access control information

### Access control on the deployed site

GitHub Pages supports restricting access to organisation members only.
Enable this under repo **Settings → Pages → Access control**.

---

## Project structure

```
ui/packages/ci-dashboard/
├── package.json                  # @consul/ci-dashboard — standalone, type: module
├── vite.config.js                # base: './' so dist/ is path-agnostic
├── index.html
├── .gitignore                    # ignores dist/, public/data/*.json
│
├── scripts/
│   └── aggregate.js              # Node 18+ script — fetches API data, writes JSON
│
├── public/
│   └── data/
│       └── .gitkeep              # directory tracked; JSON files gitignored
│
└── src/
    ├── main.js                   # state + render loop
    ├── app.css                   # design tokens + full stylesheet
    ├── utils/
    │   └── format.js             # escapeHtml, statusBadge, typeBadge, formatTimestamp
    └── components/
        ├── header.js             # sticky dark header — repo/branch/run/commit chips
        ├── summary-bar.js        # SVG donut + 5 stat cards
        ├── filters.js            # type + status pill filters
        └── job-table.js          # sortable table with expandable test sub-rows
```

---

## Adding a new branch permanently

To have a branch always appear in the nightly schedule, edit the workflow:

```yaml
# aggregate-and-deploy-dashboard.yml — "Resolve branch list" step
if [[ "${{ github.event_name }}" == "schedule" ... ]]; then
  echo "branches=main,release/1.20.x"   # ← add your branch here
```

---

## Extending the type classifier

Job type classification lives in `scripts/aggregate.js` in the
`classifyJobType()` function.  Add keywords to match your workflow naming:

```js
function classifyJobType(workflowName = '', jobName = '') {
  const combined = `${workflowName} ${jobName}`.toLowerCase();

  if (/\blint\b/.test(combined))                                      return 'Lint';
  if (/\bui\b|\bfrontend\b|\bember\b|\bplaywright\b/.test(combined))  return 'UI';
  if (/integrat|envoy|nomad|vault|consul-container|deployer/.test(combined)) return 'Integration';
  return 'Other';
}
```

---

## Test-level failure detail (check-run annotations)

When the aggregation script runs it automatically fetches **check-run annotations**
for every failed job.  This surfaces individual test names, failure messages,
and file:line locations without any changes to your existing workflows.

### How it works

GitHub Actions converts `::error file=…,line=…::message` commands written to
stdout into structured check-run annotations.  The Consul Go test suite already
emits these via:

```yaml
go run gotest.tools/gotestsum@v… \
  --format=github-actions \         # ← writes ::error:: annotations
  --junitfile ${{env.TEST_RESULTS}}/gotestsum-report.xml -- …
```

The script calls `GET /repos/{owner}/{repo}/check-runs/{id}/annotations`
for each failed job and stores the results in `job.annotations[]`.

### What you see in the dashboard

When a failed job is clicked:
- **Annotation panel** — shown when check-run failures were captured.
  Displays test name, file:line link to the exact line on GitHub, and a
  collapsible failure detail block with the full `gotestsum` output.
- **Steps panel** — shown as a fallback when no annotation data is
  available (non-test jobs, lint, build steps).
- **No breakdown** — shown when neither situation applies (e.g., network
  error during aggregation).

The summary bar's fifth card also switches from "Cases" to **"Test Failures"**
(shown in red) when annotation data is present.

### Skipping annotation fetching

Pass `--skip-annotations` to omit the extra API calls.  Useful during
development or if you have very many failed jobs:

```bash
node scripts/aggregate.js \
  --repo hashicorp/consul \
  --branch main \
  --skip-annotations
```

### Full JUnit XML (optional enrichment)

Check-run annotations only capture *failures*.  For a full pass/fail/skip
breakdown per test (not just failed ones), upload and parse the JUnit XML
that `gotestsum` already writes.  The `reusable-unit.yml` workflow uploads
it as `{RANDOM_ID}-test-results/gotestsum-report.xml`.

To enable richer data, add a download + extract step in the GitHub Actions
workflow before calling `aggregate.js`, then pass a `--junit-dir` path
(a planned future flag).  This is not required for the current feature set.

