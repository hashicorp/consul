/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import './app.css';
import { renderHeader } from './components/header.js';
import { renderSummaryBar } from './components/summary-bar.js';
import { renderFilters } from './components/filters.js';
import { renderJobTable } from './components/job-table.js';

// ---------------------------------------------------------------------------
// State
// All mutable app state lives here. Components are pure render functions that
// take state and return HTML strings. Mutations call render() to refresh.
// ---------------------------------------------------------------------------
const state = {
  runs: [],
  selectedRunIndex: 0,
  filterType: 'all',
  filterStatus: 'all',
  expandedJobId: null,
};

function activeRun() {
  return state.runs[state.selectedRunIndex];
}

function jobTypes(run) {
  return ['all', ...new Set(run.jobs.map((j) => j.type).sort())];
}

// ---------------------------------------------------------------------------
// Render
// Rebuilds the entire app from state. Fast enough for ~100 jobs.
// ---------------------------------------------------------------------------
function render() {
  const app = document.getElementById('app');
  const run = activeRun();

  app.innerHTML = `
    ${renderHeader(state)}
    <main class="main-content">
      ${renderSummaryBar(run.summary)}
      ${renderFilters(state, jobTypes(run))}
      ${renderJobTable(state, run)}
    </main>
  `;

  attachListeners();
}

function attachListeners() {
  // Run selector
  document.getElementById('run-select')?.addEventListener('change', (e) => {
    state.selectedRunIndex = parseInt(e.target.value, 10);
    state.expandedJobId = null;
    render();
  });

  // Type filter buttons
  document.querySelectorAll('[data-filter-type]').forEach((btn) => {
    btn.addEventListener('click', () => {
      state.filterType = btn.dataset.filterType;
      state.expandedJobId = null;
      render();
    });
  });

  // Status filter buttons
  document.querySelectorAll('[data-filter-status]').forEach((btn) => {
    btn.addEventListener('click', () => {
      state.filterStatus = btn.dataset.filterStatus;
      state.expandedJobId = null;
      render();
    });
  });

  // Job row expand / collapse (the tests-row is a sibling TR so clicks there
  // do NOT bubble up to the job-row — no stopPropagation needed).
  document.querySelectorAll('tr[data-job-id]').forEach((row) => {
    row.addEventListener('click', () => {
      const id = row.dataset.jobId;
      state.expandedJobId = state.expandedJobId === id ? null : id;
      render();
    });
  });
}

// ---------------------------------------------------------------------------
// Bootstrap — fetch the data file dropped by CI into public/data/
// ---------------------------------------------------------------------------
async function init() {
  const app = document.getElementById('app');
  try {
    const res = await fetch('./data/aggregated_ci_results.json');
    if (!res.ok) throw new Error(`HTTP ${res.status} — ${res.statusText}`);

    const payload = await res.json();
    // Accept both a single run object and an array of runs
    const runs = Array.isArray(payload) ? payload : [payload];
    if (!runs.length) throw new Error('No run data found in the file.');

    // Deduplicate jobs within each run by name — keep the highest-priority
    // status (passed > failed > skipped) so that re-runs and shared setup
    // steps that appear in multiple workflows don't inflate the counts.
    const STATUS_RANK = { passed: 0, failed: 1, skipped: 2 };
    for (const run of runs) {
      const byName = new Map();
      for (const job of run.jobs) {
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
      const dedupedJobs = [...byName.values()];
      if (dedupedJobs.length !== run.jobs.length) {
        console.log(`[dedup] ${run.release_version}: ${run.jobs.length} → ${dedupedJobs.length} jobs`);
        run.jobs = dedupedJobs;
        // Recompute summary counts to stay in sync with the deduplicated jobs.
        const p = dedupedJobs.filter((j) => j.status === 'passed').length;
        const f = dedupedJobs.filter((j) => j.status === 'failed').length;
        const s = dedupedJobs.filter((j) => j.status === 'skipped').length;
        const t = dedupedJobs.length;
        run.summary = {
          ...run.summary,
          total_jobs:  t,
          total_tests: t,
          passed:  p,
          failed:  f,
          skipped: s,
          pass_rate: t > 0 ? parseFloat(((p / t) * 100).toFixed(1)) : 0,
        };
      }
    }

    state.runs = runs;
    render();
  } catch (err) {
    app.innerHTML = `
      <div class="error-state">
        <div class="error-icon">⚠️</div>
        <h2>Could not load results</h2>
        <p class="error-message">${escapeHtml(err.message)}</p>
        <p class="error-hint">
          Drop <code>aggregated_ci_results.json</code> into
          <code>public/data/</code> and run <code>pnpm dev</code>.
        </p>
      </div>
    `;
  }
}

function escapeHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

init();
