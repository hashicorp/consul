export function renderSummaryBar(summary) {
  const rate = (summary.pass_rate ?? 0).toFixed(1);
  const rateNum = summary.pass_rate ?? 0;
  const rateClass = rateNum >= 95 ? 'rate-great' : rateNum >= 80 ? 'rate-good' : rateNum >= 60 ? 'rate-warn' : 'rate-bad';

  // SVG donut — r=40, circumference = 2π×40 ≈ 251.3
  const CIRC = 251.3;
  const fill = (rateNum / 100) * CIRC;
  const gap = CIRC - fill;

  return `
    <section class="summary-bar">
      <div class="summary-donut-section">
        <div class="donut-wrap ${rateClass}" style="width:92px;height:92px">
          <svg class="donut" viewBox="0 0 92 92" width="92" height="92">
            <circle class="donut-bg" cx="46" cy="46" r="40" />
            <circle
              class="donut-fill"
              cx="46" cy="46" r="40"
              stroke-dasharray="${fill} ${gap}"
              stroke-dashoffset="62.8"
            />
          </svg>
          <div class="donut-center-text">
            <span class="donut-pct">${rate}%</span>
            <span class="donut-label">Pass</span>
          </div>
        </div>

        <div class="donut-meta">
          <div class="donut-meta-item">
            <span class="donut-meta-dot dot-passed"></span>
            <span class="donut-meta-val">${summary.passed}</span>
            <span>passed</span>
          </div>
          <div class="donut-meta-item">
            <span class="donut-meta-dot dot-failed"></span>
            <span class="donut-meta-val">${summary.failed}</span>
            <span>failed</span>
          </div>
          <div class="donut-meta-item">
            <span class="donut-meta-dot dot-skipped"></span>
            <span class="donut-meta-val">${summary.skipped}</span>
            <span>skipped</span>
          </div>
        </div>
      </div>

      <div class="summary-divider"></div>

      <div class="summary-stats-section">
        <div class="stat-card stat-total">
          <span class="stat-num">${summary.total_jobs}</span>
          <span class="stat-label">Total Jobs</span>
        </div>
        <div class="stat-card stat-passed">
          <span class="stat-num">${summary.passed}</span>
          <span class="stat-label">Passed</span>
        </div>
        <div class="stat-card stat-failed">
          <span class="stat-num">${summary.failed}</span>
          <span class="stat-label">Failed</span>
        </div>
        <div class="stat-card stat-skipped">
          <span class="stat-num">${summary.skipped}</span>
          <span class="stat-label">Skipped</span>
        </div>
        <div class="stat-card stat-cases">
          ${summary.total_test_failures > 0
            ? `<span class="stat-num" style="color:var(--color-failed)">${summary.total_test_failures}</span>
               <span class="stat-label">Test Failures</span>`
            : `<span class="stat-num">${summary.total_cases ?? summary.total_tests ?? '—'}</span>
               <span class="stat-label">Cases</span>`}
        </div>
      </div>
    </section>
  `;
}
