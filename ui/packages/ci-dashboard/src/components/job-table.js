import { escapeHtml, statusBadge, typeBadge } from '../utils/format.js';

function rateClass(rate) {
  if (rate >= 95) return 'rate-fill-great';
  if (rate >= 80) return 'rate-fill-good';
  if (rate >= 60) return 'rate-fill-warn';
  return 'rate-fill-bad';
}

/** Render the annotations panel (real test-failure data from check-run annotations). */
function renderAnnotationsPanel(job, annotations, repo) {
  const rows = annotations
    .map((ann) => {
      let fileHtml = '—';
      if (ann.path) {
        const encodedPath = encodeURIComponent(ann.path).replace(/%2F/g, '/');
        const lineHash = ann.line ? `#L${ann.line}` : '';
        if (repo) {
          fileHtml = `<a href="https://github.com/${escapeHtml(repo)}/blob/HEAD/${encodedPath}${lineHash}"
                         target="_blank" rel="noopener noreferrer" class="ann-file-link">${escapeHtml(ann.path)}</a>`;
        } else {
          fileHtml = `<span class="ann-file">${escapeHtml(ann.path)}</span>`;
        }
        if (ann.line) {
          fileHtml += `<span class="ann-line-num">:${ann.line}</span>`;
        }
      }
      const messageHtml = ann.message
        ? `<details class="ann-detail">
             <summary class="ann-summary">show details</summary>
             <pre class="ann-pre">${escapeHtml(ann.message)}</pre>
           </details>`
        : '<span class="ann-no-detail">—</span>';

      return `
        <tr class="ann-row ann-level-${escapeHtml(ann.level ?? 'failure')}">
          <td class="ann-name-cell">
            <span class="ann-dot"></span>
            <span class="ann-name">${escapeHtml(ann.name)}</span>
          </td>
          <td class="ann-file-cell">${fileHtml}</td>
          <td class="ann-message-cell">${messageHtml}</td>
        </tr>`;
    })
    .join('');

  return `
    <tr class="tests-row">
      <td colspan="7">
        <div class="annotations-inner">
          <div class="ann-header">
            <span class="ann-title">
              Failed Tests
              <span class="ann-count">${annotations.length}</span>
            </span>
            <span class="ann-source">via check-run annotations</span>
          </div>
          <table class="ann-table">
            <thead>
              <tr>
                <th class="ann-th">Test</th>
                <th class="ann-th">Location</th>
                <th class="ann-th">Detail</th>
              </tr>
            </thead>
            <tbody>${rows}</tbody>
          </table>
        </div>
      </td>
    </tr>`;
}

/** Render the step-cases panel (fallback when no annotation data is available). */
function renderStepsPanel(job) {
  const cases = (job.tests?.[0]?.cases ?? []);
  if (cases.length === 0) return '';

  const rows = cases
    .map(
      (c) => `
      <tr class="test-row status-${c.status}">
        <td>
          <div class="test-name-block">
            <span class="test-name">${escapeHtml(c.name)}</span>
          </div>
        </td>
        <td>${statusBadge(c.status)}</td>
        <td class="cell-right">${c.duration_seconds ?? 0}s</td>
      </tr>`,
    )
    .join('');

  return `
    <tr class="tests-row">
      <td colspan="7">
        <div class="tests-inner">
          <table class="tests-table">
            <thead>
              <tr>
                <th>Step</th>
                <th>Status</th>
                <th class="th-right">Duration</th>
              </tr>
            </thead>
            <tbody>${rows}</tbody>
          </table>
        </div>
      </td>
    </tr>`;
}

export function renderJobTable(state, run) {
  let jobs = run.jobs;

  if (state.filterType !== 'all') {
    jobs = jobs.filter((j) => j.type === state.filterType);
  }
  if (state.filterStatus !== 'all') {
    jobs = jobs.filter((j) => j.status === state.filterStatus);
  }

  if (jobs.length === 0) {
    return `
      <section class="table-section">
        <div class="empty-state">
          <div class="empty-state-icon">🔍</div>
          <div class="empty-state-title">No jobs match these filters</div>
          <div class="empty-state-sub">Try changing the type or status filter above.</div>
        </div>
      </section>`;
  }

  const failedCount  = jobs.filter((j) => j.status === 'failed').length;
  const skippedCount = jobs.filter((j) => j.status === 'skipped').length;

  const rows = jobs
    .map((job) => {
      const expanded = state.expandedJobId === job.job_id;
      const rate = (job.stats?.pass_rate ?? 0).toFixed(0);
      const barWidth = Math.max(0, Math.min(100, job.stats?.pass_rate ?? 0));
      const statusRowClass = job.status === 'failed' ? ' row-failed' : job.status === 'skipped' ? ' row-skipped' : '';
      const fillCls = rateClass(job.stats?.pass_rate ?? 0);

      const annotations = job.annotations ?? [];
      const stepCases   = job.tests?.[0]?.cases ?? [];

      // Annotation count badge shown in the job name column for failing jobs with test data.
      const annotBadge = annotations.length > 0
        ? `<span class="annot-badge">${annotations.length} test failure${annotations.length !== 1 ? 's' : ''}</span>`
        : '';

      let expandedContent = '';
      if (expanded) {
        if (annotations.length > 0) {
          expandedContent = renderAnnotationsPanel(job, annotations, run.repo);
        } else if (stepCases.length > 0) {
          expandedContent = renderStepsPanel(job);
        } else {
          expandedContent = `
            <tr class="tests-row">
              <td colspan="7">
                <div class="no-tests">
                  No individual test breakdown available for this job.
                  ${job.status === 'failed' && run.summary?.annotations_fetched === false
                    ? '<span class="hint"> Re-run aggregation without <code>--skip-annotations</code> to fetch test details.</span>'
                    : ''}
                </div>
              </td>
            </tr>`;
        }
      }

      return `
        <tr class="job-row${statusRowClass}${expanded ? ' expanded' : ''}" data-job-id="${escapeHtml(job.job_id)}">
          <td class="td-name">
            <div class="job-name-inner">
              <span class="chevron">${expanded ? '▾' : '▸'}</span>
              <span class="job-name-text">${escapeHtml(job.name)}</span>
              ${annotBadge}
            </div>
          </td>
          <td>${typeBadge(job.type)}</td>
          <td>${statusBadge(job.status)}</td>
          <td class="td-num">${job.stats?.passed ?? 0}</td>
          <td class="td-num ${job.stats?.failed > 0 ? 'cell-failed' : ''}">${job.stats?.failed ?? 0}</td>
          <td class="td-num">${job.stats?.skipped ?? 0}</td>
          <td class="td-rate">
            <div class="rate-cell">
              <div class="rate-bar">
                <div class="rate-fill ${fillCls}" style="width:${barWidth}%"></div>
              </div>
              <span class="rate-pct">${rate}%</span>
            </div>
          </td>
        </tr>
        ${expandedContent}
      `;
    })
    .join('');

  // Total test failures across all displayed jobs
  const totalTestFailures = jobs.reduce((acc, j) => acc + (j.annotation_count ?? 0), 0);

  const footerRight = failedCount > 0
    ? `<span style="color:var(--color-failed);font-weight:600">${failedCount} job${failedCount !== 1 ? 's' : ''} failed${totalTestFailures > 0 ? ` · ${totalTestFailures} test failure${totalTestFailures !== 1 ? 's' : ''}` : ''}</span>`
    : skippedCount > 0
    ? `<span>${skippedCount} skipped</span>`
    : `<span style="color:var(--color-passed)">All passing ✓</span>`;

  return `
    <section class="table-section">
      <div class="table-toolbar">
        <span class="table-title">Jobs</span>
        <span class="table-count">${jobs.length} of ${run.jobs.length}</span>
      </div>
      <table class="job-table">
        <thead>
          <tr>
            <th>Job</th>
            <th>Type</th>
            <th>Status</th>
            <th class="th-right">Passed</th>
            <th class="th-right">Failed</th>
            <th class="th-right">Skipped</th>
            <th>Pass Rate</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
      <div class="table-footer">
        <span>Showing <strong>${jobs.length}</strong> of <strong>${run.jobs.length}</strong> jobs</span>
        ${footerRight}
      </div>
    </section>
  `;
}

