import { escapeHtml, formatTimestamp } from '../utils/format.js';

export function renderHeader(state) {
  const run = state.runs[state.selectedRunIndex];
  const sha = String(run.commit_sha ?? '').slice(0, 7);
  const ts = formatTimestamp(run.timestamp);

  const runOptions = state.runs
    .map(
      (r, i) =>
        `<option value="${i}" ${i === state.selectedRunIndex ? 'selected' : ''}>
          ${escapeHtml(r.release_version)}  ·  ${escapeHtml(r.branch)}
        </option>`
    )
    .join('');

  const controls =
    state.runs.length > 1
      ? `<span class="run-selector-label">Run</span>
         <select id="run-select" class="run-select">${runOptions}</select>`
      : `<span class="meta-chip-value">${escapeHtml(run.release_version)}</span>`;

  return `
    <header class="app-header">
      <div class="header-brand-block">
        <div class="brand-icon">C</div>
        <span class="brand-name">Consul</span>
        <span class="brand-slash">/</span>
        <span class="brand-sub">CI Dashboard</span>
      </div>

      <div class="header-meta-block">
        <div class="meta-chip">
          <span class="meta-chip-label">Repo</span>
          <span class="meta-chip-value">${escapeHtml(run.repo)}</span>
        </div>
        <div class="meta-chip">
          <span class="meta-chip-label">Branch</span>
          <span class="meta-chip-value">${escapeHtml(run.release_branch ?? run.branch)}</span>
        </div>
        <div class="meta-chip">
          <span class="meta-chip-label">Run</span>
          <span class="meta-chip-value mono">${escapeHtml(run.run_id)}</span>
        </div>
        <div class="meta-chip">
          <span class="meta-chip-label">Commit</span>
          <span class="meta-chip-value mono">${escapeHtml(sha)}</span>
        </div>
        <div class="meta-chip">
          <span class="meta-chip-label">Generated</span>
          <span class="meta-chip-value">${escapeHtml(ts)}</span>
        </div>
      </div>

      <div class="header-controls-block">${controls}</div>
    </header>
  `;
}
