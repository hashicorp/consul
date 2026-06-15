#!/usr/bin/env node
// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1
//
// aggregate.js  —  Fetch GitHub Actions workflow run data and produce the
//                  aggregated_ci_results.json consumed by the CI dashboard.
//
// Two modes:
//
//   Branch mode (default)  — fetches workflow runs for a branch:
//     node scripts/aggregate.js \
//       --repo  hashicorp/consul \
//       --branch  main \
//       --label  "nightly-main" \
//       --release-branch  main \
//       --release-version  "main-$(date +%Y%m%d)" \
//       --output  public/data/aggregated_ci_results.json
//
//   Tag mode (RC releases)  — fetches check-runs by RC tag commit SHA:
//     node scripts/aggregate.js \
//       --repo  hashicorp/consul \
//       --tag-pattern  v2.0 \
//       --output  public/data/aggregated_ci_results.json
//
// Environment:
//   GITHUB_TOKEN  — a token with `repo` (or `public_repo`) read scope.
//                   In GitHub Actions this is automatically available as
//                   secrets.GITHUB_TOKEN; no extra secret needed.
//
// Security note:
//   The token is read from the environment and used only for API calls.
//   It is NEVER written into the output JSON.  The output contains only
//   job names, statuses, pass rates, and commit SHAs — no credentials.
//
// No third-party dependencies.  Requires Node ≥ 18 (built-in fetch).

import { writeFileSync, readFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { parseArgs } from 'node:util';

// ─── CLI args ─────────────────────────────────────────────────────────────
const { values: args } = parseArgs({
  options: {
    repo:            { type: 'string' },
    branch:          { type: 'string' },
    label:           { type: 'string' },
    'release-branch':  { type: 'string' },
    'release-version': { type: 'string' },
    output:          { type: 'string', default: 'public/data/aggregated_ci_results.json' },
    'max-runs':         { type: 'string',  default: '50' },   // max workflow runs to fetch per page
    'existing':         { type: 'string',  default: '' },     // path to existing JSON to append to (optional)
    'skip-annotations': { type: 'boolean', default: false },  // skip check-run annotation fetching (faster but less detail)
    'tag-pattern':      { type: 'string',  default: '' },     // tag prefix for RC mode, e.g. "v2.0"
  },
  strict: true,
  allowPositionals: false,
});

// In tag mode --branch is not required; in branch mode --branch IS required.
const TAG_PATTERN = args['tag-pattern'];
if (!args['repo']) {
  console.error('Error: --repo is required.');
  process.exit(1);
}
if (!TAG_PATTERN && !args['branch']) {
  console.error('Error: either --branch or --tag-pattern is required.');
  process.exit(1);
}

// ─── Token — read once, never log, clear the named binding after use ─────
const _rawToken = process.env.GITHUB_TOKEN;
if (!_rawToken) {
  console.error('Error: GITHUB_TOKEN environment variable is not set.');
  process.exit(1);
}

// Build auth headers immediately and drop the raw token reference so it
// cannot be accidentally serialised or logged anywhere below this point.
const _HEADERS = Object.freeze({
  'Authorization': `Bearer ${_rawToken}`,
  'Accept': 'application/vnd.github+json',
  'X-GitHub-Api-Version': '2022-11-28',
  'User-Agent': 'consul-ci-dashboard/1.0',
});
// Intentionally shadow the raw token to make the value inaccessible by name.
const _rawToken_cleared = undefined; // eslint-disable-line no-unused-vars

// ─── Redaction helper ─────────────────────────────────────────────────────
// Replaces the token (and its URL-encoded form) anywhere it appears in a
// string.  Called on every string that is logged or thrown as an error so
// the token can never surface in stdout, stderr, or CI logs.
function redact(str) {
  // Match Bearer prefix too in case the header value is ever echoed back.
  return String(str)
    .replace(/Bearer\s+[A-Za-z0-9_\-\.]+/g, 'Bearer [REDACTED]')
    .replace(/ghp_[A-Za-z0-9]+/g, '[REDACTED]')
    .replace(/ghs_[A-Za-z0-9]+/g, '[REDACTED]')
    .replace(/github_pat_[A-Za-z0-9_]+/g, '[REDACTED]');
}

// ─── Input validation ─────────────────────────────────────────────────────
// Validate inputs that end up in URLs or file paths before using them.
function validateRepo(repo) {
  // Must be «owner/repo» with safe characters only — prevents URL injection.
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repo)) {
    console.error(`Error: --repo "${repo}" is not a valid owner/repo value.`);
    process.exit(1);
  }
}

function validateBranch(branch) {
  // Branches can contain letters, numbers, / . - _ but not shell metacharacters.
  if (!/^[A-Za-z0-9/._\-]+$/.test(branch)) {
    console.error(`Error: --branch "${branch}" contains invalid characters.`);
    process.exit(1);
  }
}

// ─── Runtime inputs ───────────────────────────────────────────────────────
const REPO  = args['repo'];
validateRepo(REPO);

const BRANCH = args['branch'] ?? '';
if (BRANCH) validateBranch(BRANCH);

// Tag-pattern validation — only printable ASCII, no shell metacharacters.
if (TAG_PATTERN && !/^[A-Za-z0-9._\-/]+$/.test(TAG_PATTERN)) {
  console.error(`Error: --tag-pattern "${TAG_PATTERN}" contains invalid characters.`);
  process.exit(1);
}

const LABEL           = args['label']            ?? BRANCH;
const RELEASE_BRANCH  = args['release-branch']   ?? BRANCH;
const RELEASE_VERSION = args['release-version']  ?? BRANCH;
const OUTPUT          = args['output'];
const MAX_RUNS        = Math.min(parseInt(args['max-runs'], 10), 100);
const EXISTING_PATH   = args['existing'];
const SKIP_ANNOTATIONS = args['skip-annotations'] ?? false;

// ─── HTTP helper ──────────────────────────────────────────────────────────
const BASE = 'https://api.github.com';

async function ghFetch(path) {
  const url = `${BASE}${path}`;
  let res;
  try {
    res = await fetch(url, { headers: _HEADERS });
  } catch (networkErr) {
    // Network errors from fetch() never contain token data, but redact anyway.
    throw new Error(`Network error fetching ${url}: ${redact(networkErr.message)}`);
  }
  if (!res.ok) {
    // API error bodies are redacted before being included in the thrown message
    // because GitHub sometimes echoes request context in 4xx responses.
    const body = await res.text().catch(() => '');
    throw new Error(`GitHub API ${res.status} for ${url}: ${redact(body.slice(0, 200))}`);
  }
  return res.json();
}

// Paginate through all pages of a list endpoint (up to maxItems items).
async function ghFetchAll(path, arrayKey, maxItems = 500) {
  const items = [];
  let page = 1;
  while (items.length < maxItems) {
    const perPage = Math.min(100, maxItems - items.length);
    const data = await ghFetch(`${path}${path.includes('?') ? '&' : '?'}per_page=${perPage}&page=${page}`);
    const batch = data[arrayKey] ?? [];
    items.push(...batch);
    if (batch.length < perPage) break; // last page
    page++;
  }
  return items;
}

// ─── Tag-based helpers (RC mode) ────────────────────────────────────────────

/**
 * Fetch all git refs that match a tag prefix, e.g. "v2.0" matches v2.0.0-rc1 etc.
 * Uses /repos/{owner}/{repo}/git/matching-refs/tags/{pattern}.
 */
async function fetchTagRefs(pattern) {
  return ghFetch(`/repos/${REPO}/git/matching-refs/tags/${encodeURIComponent(pattern)}`);
}

/**
 * Fetch all check runs for a specific commit SHA.
 * Paginates automatically; returns the check_runs array.
 */
async function fetchCheckRunsByCommit(sha) {
  return ghFetchAll(
    `/repos/${REPO}/commits/${encodeURIComponent(sha)}/check-runs`,
    'check_runs',
    500,
  );
}

/**
 * Build a dashboard run object from check-runs fetched by commit SHA.
 * Mirrors the branch-mode job-mapping logic so the output schema is identical.
 */
function buildRunObjectFromCheckRuns(checkRuns) {
  // Status priority for deduplication: lower rank wins.
  const STATUS_RANK = { passed: 0, failed: 1, skipped: 2 };

  // First pass: map all check-runs to dashboard job shape.
  const allJobs = checkRuns.map((cr) => {
    const status  = mapStatus(cr.conclusion);
    const type    = classifyJobType(cr.app?.slug ?? '', cr.name);

    let durationSeconds = 0;
    if (cr.started_at && cr.completed_at) {
      durationSeconds = Math.round(
        (new Date(cr.completed_at) - new Date(cr.started_at)) / 1000,
      );
    }

    const stats = {
      total:            1,
      passed:           status === 'passed'  ? 1 : 0,
      failed:           status === 'failed'  ? 1 : 0,
      skipped:          status === 'skipped' ? 1 : 0,
      duration_seconds: durationSeconds,
      pass_rate:        status === 'passed'  ? 100 : 0,
    };

    return {
      job_id:           String(cr.id),
      name:             cr.name,
      type,
      status,
      stats,
      annotations:      [],
      annotation_count: 0,
      tests: [
        {
          test_id: `${cr.id}-test`,
          name:    cr.name,
          package: REPO,
          status,
          stats,
          cases:   [],
        },
      ],
    };
  });

  // Second pass: deduplicate by name, keeping the highest-priority status.
  // GitHub returns the same check name from multiple workflows on the same commit.
  const byName = new Map();
  for (const job of allJobs) {
    const existing = byName.get(job.name);
    if (!existing) {
      byName.set(job.name, job);
    } else {
      const rank = (s) => STATUS_RANK[s] ?? 3;
      if (rank(job.status) < rank(existing.status)) {
        byName.set(job.name, job);
      }
    }
  }

  const dashboardJobs = [...byName.values()];
  if (dashboardJobs.length !== allJobs.length) {
    console.log(`  Deduped ${allJobs.length} check-runs → ${dashboardJobs.length} unique job names`);
  }

  return dashboardJobs;
}

// ─── Tag mode — main flow ─────────────────────────────────────────────────

async function runTagMode() {
  console.log(`[tag mode]  Fetching RC tags for ${REPO}  pattern="${TAG_PATTERN}"`);

  const allRefs = await fetchTagRefs(TAG_PATTERN);

  // Keep only refs containing "-rc" (case-insensitive), like the Python script.
  const rcRefs = allRefs.filter((r) => r.ref.toLowerCase().includes('-rc'));

  if (rcRefs.length === 0) {
    console.error(`No RC tags found matching "${TAG_PATTERN}"`);
    process.exit(1);
  }

  console.log(`Found ${rcRefs.length} RC tag(s): ${rcRefs.map((r) => r.ref.replace('refs/tags/', '')).join(', ')}`);

  const runObjects = [];
  const timestamp = new Date().toISOString();

  for (const ref of [...rcRefs].sort((a, b) => a.ref.localeCompare(b.ref))) {
    const tagName = ref.ref.replace('refs/tags/', '');
    const sha     = ref.object.sha;
    console.log(`\nProcessing ${tagName}  (${sha.slice(0, 12)})`);

    let checkRuns;
    try {
      checkRuns = await fetchCheckRunsByCommit(sha);
    } catch (err) {
      console.warn(`  Warning: could not fetch check-runs for ${tagName}: ${redact(err.message)}`);
      continue;
    }
    console.log(`  ${checkRuns.length} check run(s)`);

    const dashboardJobs = buildRunObjectFromCheckRuns(checkRuns);

    // Enrich failed jobs with annotations (same logic as branch mode).
    if (!SKIP_ANNOTATIONS) {
      const failedJobs = dashboardJobs.filter((j) => j.status === 'failed');
      if (failedJobs.length > 0) {
        let annotTotal = 0;
        const ANNOT_BATCH = 5;
        for (let i = 0; i < failedJobs.length; i += ANNOT_BATCH) {
          await Promise.all(
            failedJobs.slice(i, i + ANNOT_BATCH).map(async (dashJob) => {
              try {
                const raw = await fetchAnnotations(dashJob.job_id);
                const failures = raw.filter(
                  (a) => a.annotation_level === 'failure' || a.annotation_level === 'warning',
                );
                if (failures.length > 0) {
                  dashJob.annotations = failures.map((ann, idx) => ({
                    id:      `ann-${dashJob.job_id}-${idx}`,
                    name:    extractAnnotationTestName(ann),
                    path:    ann.path ?? null,
                    line:    ann.start_line ?? null,
                    message: extractAnnotationDetail(ann),
                    level:   ann.annotation_level,
                  }));
                  dashJob.annotation_count = failures.length;
                  annotTotal += failures.length;
                }
              } catch (err) {
                console.warn(`  Warning: annotations unavailable for "${dashJob.name}": ${redact(err.message)}`);
              }
            }),
          );
        }
        if (annotTotal > 0) {
          console.log(`  ${annotTotal} test failure annotation(s) found`);
        }
      }
    }

    const passed  = dashboardJobs.filter((j) => j.status === 'passed').length;
    const failed  = dashboardJobs.filter((j) => j.status === 'failed').length;
    const skipped = dashboardJobs.filter((j) => j.status === 'skipped').length;
    const total   = dashboardJobs.length;
    const totalAnnotations = dashboardJobs.reduce((acc, j) => acc + j.annotation_count, 0);

    const summary = {
      total_jobs:          total,
      total_tests:         total,
      total_cases:         0,
      total_test_failures: totalAnnotations,
      annotations_fetched: !SKIP_ANNOTATIONS,
      passed,
      failed,
      skipped,
      duration_seconds: dashboardJobs.reduce((acc, j) => acc + j.stats.duration_seconds, 0),
      pass_rate: total > 0 ? parseFloat(((passed / total) * 100).toFixed(1)) : 0,
    };

    console.log(`  ${total} jobs  |  ${passed} passed  |  ${failed} failed  |  ${skipped} skipped  |  pass rate ${summary.pass_rate}%`);

    runObjects.push({
      release_branch:  TAG_PATTERN,
      release_version: tagName,
      repo:            REPO,
      run_id:          `Tag-${tagName}`,
      branch:          tagName,
      commit_sha:      sha,
      timestamp,
      summary,
      jobs: dashboardJobs,
    });
  }

  if (runObjects.length === 0) {
    console.error('No run objects produced — aborting.');
    process.exit(1);
  }

  // Optionally merge with existing file.
  let output = runObjects;
  if (EXISTING_PATH) {
    try {
      const resolvedExisting = resolve(EXISTING_PATH);
      const existing = JSON.parse(readFileSync(resolvedExisting, 'utf8'));
      const arr = Array.isArray(existing) ? existing : [existing];
      for (const runObj of runObjects) {
        const idx = arr.findIndex((r) => r.release_version === runObj.release_version);
        if (idx >= 0) arr[idx] = runObj; else arr.push(runObj);
      }
      output = arr;
    } catch {
      output = runObjects;
    }
  }

  mkdirSync(dirname(OUTPUT), { recursive: true });
  writeFileSync(OUTPUT, JSON.stringify(output, null, 2), 'utf8');
  console.log(`\nWrote ${OUTPUT}  (${runObjects.length} RC tag run(s))`);
}

// ─── Check-run annotation helpers ────────────────────────────────────────────

/** Extract the numeric check-run ID from a check_run_url field on a job. */
function extractCheckRunId(checkRunUrl) {
  if (!checkRunUrl || typeof checkRunUrl !== 'string') return null;
  const m = checkRunUrl.match(/\/check-runs\/(\d+)(?:$|\?)/);
  return m ? m[1] : null;
}

/**
 * Fetch all annotations for a check run.
 * The endpoint returns a JSON array directly (not wrapped in an object key)
 * so we cannot reuse ghFetchAll — we paginate manually.
 */
async function fetchAnnotations(checkRunId) {
  const items = [];
  let page = 1;
  while (true) {
    const url = `${BASE}/repos/${REPO}/check-runs/${checkRunId}/annotations?per_page=100&page=${page}`;
    let res;
    try {
      res = await fetch(url, { headers: _HEADERS });
    } catch (networkErr) {
      throw new Error(`Network error fetching annotations: ${redact(networkErr.message)}`);
    }
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`GitHub API ${res.status} for annotations: ${redact(body.slice(0, 200))}`);
    }
    const batch = await res.json();
    if (!Array.isArray(batch) || batch.length === 0) break;
    items.push(...batch);
    if (batch.length < 100) break;
    page++;
  }
  return items;
}

/** Extract a clean test name from a GitHub check-run annotation. */
function extractAnnotationTestName(ann) {
  // gotestsum --format=github-actions puts the test name in the title field.
  const title = (ann.title ?? '').trim();
  if (/^Test[A-Z_]/.test(title)) return title;

  // Also try the first line of message (some tools use that).
  const firstLine = (ann.message ?? '').split('\n')[0].trim();
  if (/^Test[A-Z_]/.test(firstLine)) return firstLine;

  // Fall back: look for "--- FAIL: TestXxx" pattern in the message body.
  const failMatch = (ann.message ?? '').match(/---\s*(?:FAIL|ERROR):\s+(Test\S+)/);
  if (failMatch) return failMatch[1];

  // Last resort: use title or first message line verbatim.
  return title || firstLine || 'Unknown test';
}

/** Extract failure detail lines from an annotation message (strips test-name header). */
function extractAnnotationDetail(ann) {
  return (ann.message ?? '')
    .split('\n')
    // Drop lines that are just the test name or a PASS/FAIL marker.
    .filter((l) => l.trim() && !/^---\s*(FAIL|PASS):/.test(l.trim()))
    .join('\n')
    .trim();
}

// ─── Type classification ───────────────────────────────────────────────────
// Classifies a job into one of the four dashboard types based on the GitHub
// workflow file name and job name.  Adjust the keywords to match your
// organisation's naming conventions.
function classifyJobType(workflowName = '', jobName = '') {
  const combined = `${workflowName} ${jobName}`.toLowerCase();

  if (/\blint\b/.test(combined))                                    return 'Lint';
  if (/\bui\b|\bfrontend\b|\bember\b|\bplaywright\b/.test(combined)) return 'UI';
  if (/integrat|envoy|nomad|vault|consul-container|deployer/.test(combined)) return 'Integration';
  return 'Other';
}

// ─── Status mapping ────────────────────────────────────────────────────────
// Maps GitHub job/run conclusion strings to the dashboard status values.
function mapStatus(conclusion) {
  switch (conclusion) {
    case 'success':   return 'passed';
    case 'failure':   return 'failed';
    case 'cancelled':
    case 'skipped':
    case 'neutral':   return 'skipped';
    case 'timed_out': return 'failed';
    default:          return conclusion ?? 'skipped';
  }
}

// ─── Main ──────────────────────────────────────────────────────────────────
async function main() {
  console.log(`Fetching workflow runs for ${REPO}  branch=${BRANCH}  (max ${MAX_RUNS})`);

  // 1. Get the most recent completed workflow runs for this branch.
  const runs = await ghFetchAll(
    `/repos/${REPO}/actions/runs?branch=${encodeURIComponent(BRANCH)}&status=completed`,
    'workflow_runs',
    MAX_RUNS,
  );

  if (runs.length === 0) {
    console.error(`No completed workflow runs found for branch "${BRANCH}".`);
    process.exit(1);
  }

  // Use the most recent run's commit SHA and timestamp as the "snapshot" metadata.
  const latestRun = runs[0];
  const commitSha = latestRun.head_sha;
  const timestamp = new Date().toISOString();

  console.log(`Found ${runs.length} workflow runs.  Latest commit: ${commitSha.slice(0, 12)}`);

  // 2. For each workflow run, fetch its jobs.
  //    We do this concurrently in batches of 8 to be polite to the API.
  const BATCH = 8;
  const allJobs = [];

  for (let i = 0; i < runs.length; i += BATCH) {
    const batch = runs.slice(i, i + BATCH);
    const results = await Promise.all(
      batch.map(async (run) => {
        try {
          const jobs = await ghFetchAll(
            `/repos/${REPO}/actions/runs/${run.id}/jobs`,
            'jobs',
            200,
          );
          return jobs.map((job) => ({ job, workflowName: run.name }));
        } catch (err) {
          // Redact before logging — API error messages may echo request context.
          console.warn(`  Warning: could not fetch jobs for run ${run.id}: ${redact(err.message)}`);
          return [];
        }
      }),
    );
    allJobs.push(...results.flat());
    process.stdout.write(`  Fetched jobs for ${Math.min(i + BATCH, runs.length)}/${runs.length} runs\r`);
  }
  console.log(`\n  Total jobs collected: ${allJobs.length}`);

  // 3. Deduplicate by job name (keep most recent occurrence — runs are newest-first).
  //    This ensures each logical job appears once in the summary.
  const seen = new Set();
  const dedupedJobs = [];
  for (const { job, workflowName } of allJobs) {
    if (!seen.has(job.name)) {
      seen.add(job.name);
      dedupedJobs.push({ job, workflowName });
    }
  }
  console.log(`  After deduplication: ${dedupedJobs.length} unique jobs`);

  // 4. Map each job to the dashboard schema.
  const dashboardJobs = dedupedJobs.map(({ job, workflowName }) => {
    const status  = mapStatus(job.conclusion);
    const type    = classifyJobType(workflowName, job.name);

    // Duration — GitHub provides started_at / completed_at ISO strings
    let durationSeconds = 0;
    if (job.started_at && job.completed_at) {
      durationSeconds = Math.round(
        (new Date(job.completed_at) - new Date(job.started_at)) / 1000,
      );
    }

    const stats = {
      total:            1,
      passed:           status === 'passed'  ? 1 : 0,
      failed:           status === 'failed'  ? 1 : 0,
      skipped:          status === 'skipped' ? 1 : 0,
      duration_seconds: durationSeconds,
      pass_rate:        status === 'passed'  ? 100 : 0,
    };

    // Map individual steps as "cases" if there are any non-trivial steps.
    const cases = (job.steps ?? [])
      .filter((s) => s.name !== 'Set up job' && s.name !== 'Complete job')
      .map((step) => ({
        case_id:          `${job.id}-${step.number}`,
        name:             step.name,
        status:           mapStatus(step.conclusion),
        duration_seconds: (() => {
          if (step.started_at && step.completed_at) {
            return Math.round((new Date(step.completed_at) - new Date(step.started_at)) / 1000);
          }
          return 0;
        })(),
      }));

    return {
      job_id:           String(job.id),
      name:             job.name,
      type,
      status,
      stats,
      // annotations: populated below for failed jobs once check-run data is fetched.
      annotations:      [],
      annotation_count: 0,
      tests: [
        {
          test_id:  `${job.id}-test`,
          name:     job.name,
          package:  REPO,
          status,
          stats,
          cases,
        },
      ],
    };
  });

  // 4b. Enrich failed jobs with check-run annotations (individual test failures).
  //     Each annotation = one failed test case; message contains the full failure output.
  //     This uses GitHub's annotation API which is populated automatically when
  //     gotestsum is invoked with --format=github-actions.
  if (!SKIP_ANNOTATIONS) {
    const failedDashJobs = dashboardJobs.filter((j) => j.status === 'failed');
    if (failedDashJobs.length > 0) {
      console.log(`\nFetching check-run annotations for ${failedDashJobs.length} failed job(s)...`);

      // Build a lookup: "job_id" → original GitHub job object (for check_run_url).
      const jobById = new Map(dedupedJobs.map(({ job }) => [String(job.id), job]));

      let annotJobsFetched = 0;
      let annotTotal = 0;
      const ANNOT_BATCH = 5; // conservative batch size

      for (let i = 0; i < failedDashJobs.length; i += ANNOT_BATCH) {
        const batch = failedDashJobs.slice(i, i + ANNOT_BATCH);
        await Promise.all(
          batch.map(async (dashJob) => {
            const origJob = jobById.get(dashJob.job_id);
            if (!origJob) return;
            const checkRunId = extractCheckRunId(origJob.check_run_url);
            if (!checkRunId) return;

            try {
              const raw = await fetchAnnotations(checkRunId);
              const failures = raw.filter((a) => a.annotation_level === 'failure' || a.annotation_level === 'warning');
              if (failures.length > 0) {
                dashJob.annotations = failures.map((ann, idx) => ({
                  id:      `ann-${checkRunId}-${idx}`,
                  name:    extractAnnotationTestName(ann),
                  path:    ann.path ?? null,
                  line:    ann.start_line ?? null,
                  message: extractAnnotationDetail(ann),
                  level:   ann.annotation_level,
                }));
                dashJob.annotation_count = failures.length;
                annotTotal += failures.length;
              }
              annotJobsFetched++;
            } catch (err) {
              console.warn(`  Warning: annotations unavailable for "${dashJob.name}": ${redact(err.message)}`);
            }
          }),
        );
        process.stdout.write(
          `  Annotations: ${Math.min(i + ANNOT_BATCH, failedDashJobs.length)}/${failedDashJobs.length} jobs processed\r`,
        );
      }

      if (annotJobsFetched > 0) {
        console.log(`\n  ${annotTotal} test failure annotation(s) across ${annotJobsFetched} job(s)`);
      } else {
        console.log('\n  No annotations found (check-runs may not emit annotations for this workflow)');
      }
    }
  } else {
    console.log('  Skipping annotation fetch (--skip-annotations)');
  }

  // 5. Build the summary.
  const passed  = dashboardJobs.filter((j) => j.status === 'passed').length;
  const failed  = dashboardJobs.filter((j) => j.status === 'failed').length;
  const skipped = dashboardJobs.filter((j) => j.status === 'skipped').length;
  const total   = dashboardJobs.length;
  const totalCases = dashboardJobs.reduce((acc, j) => acc + j.tests[0].cases.length, 0);
  const totalAnnotations = dashboardJobs.reduce((acc, j) => acc + j.annotation_count, 0);

  const summary = {
    total_jobs:          total,
    total_tests:         total,
    total_cases:         totalCases,
    total_test_failures: totalAnnotations,
    annotations_fetched: !SKIP_ANNOTATIONS,
    passed,
    failed,
    skipped,
    duration_seconds: dashboardJobs.reduce((acc, j) => acc + j.stats.duration_seconds, 0),
    pass_rate:        total > 0 ? parseFloat(((passed / total) * 100).toFixed(1)) : 0,
  };

  // 6. Build the final run object.
  const runObject = {
    release_branch:  RELEASE_BRANCH,
    release_version: RELEASE_VERSION,
    repo:            REPO,
    run_id:          LABEL,
    branch:          BRANCH,
    commit_sha:      commitSha,
    timestamp,
    summary,
    jobs: dashboardJobs,
  };

  // 7. Optionally merge with an existing results file (multi-branch / multi-run array).
  let output;
  if (EXISTING_PATH) {
    try {
      // resolve() normalises the path so relative traversal (../../etc) is
      // visible in logs and doesn't silently escape the working directory.
      const resolvedExisting = resolve(EXISTING_PATH);
      const existing = JSON.parse(readFileSync(resolvedExisting, 'utf8'));
      const arr = Array.isArray(existing) ? existing : [existing];
      // Replace the entry with the same release_version if it already exists.
      const idx = arr.findIndex((r) => r.release_version === RELEASE_VERSION);
      if (idx >= 0) {
        arr[idx] = runObject;
      } else {
        arr.push(runObject);
      }
      output = arr;
    } catch {
      output = [runObject];
    }
  } else {
    output = [runObject];
  }

  // 8. Write output.
  mkdirSync(dirname(OUTPUT), { recursive: true });
  writeFileSync(OUTPUT, JSON.stringify(output, null, 2), 'utf8');

  console.log(`\nWrote ${OUTPUT}`);
  console.log(`  ${total} jobs  |  ${passed} passed  |  ${failed} failed  |  ${skipped} skipped  |  pass rate ${summary.pass_rate}%`);
  if (totalAnnotations > 0) {
    console.log(`  ${totalAnnotations} individual test failure(s) captured via check-run annotations`);
  } else if (totalCases > 0) {
    console.log(`  ${totalCases} step-level cases extracted`);
  }
}

// ─── Process-level safety net ────────────────────────────────────────────
// Catches anything that escapes main().catch() (e.g. async microtask throws)
// and ensures the token is never printed via an uncaught rejection dump.
process.on('unhandledRejection', (reason) => {
  const msg = reason instanceof Error ? reason.message : String(reason);
  console.error('\nUnhandled rejection:', redact(msg));
  process.exit(1);
});

process.on('uncaughtException', (err) => {
  console.error('\nUncaught exception:', redact(err.message));
  process.exit(1);
});

// Dispatch to the appropriate mode.
const _entryFn = TAG_PATTERN ? runTagMode : main;
_entryFn().catch((err) => {
  console.error('\nFatal:', redact(err.message));
  process.exit(1);
});
