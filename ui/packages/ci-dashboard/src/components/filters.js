export function renderFilters(state, types) {
  const typeButtons = types
    .map((type) => {
      const active = state.filterType === type;
      const label = type === 'all' ? 'All Types' : type;
      return `<button
        class="filter-btn${active ? ' active' : ''}"
        data-filter-type="${type}"
      >${label}</button>`;
    })
    .join('');

  const statuses = ['all', 'passed', 'failed', 'skipped'];
  const statusButtons = statuses
    .map((s) => {
      const active = state.filterStatus === s;
      const label = s === 'all' ? 'All' : s.charAt(0).toUpperCase() + s.slice(1);
      const statusCls = s !== 'all' ? ` status-${s}` : '';
      return `<button
        class="filter-btn${statusCls}${active ? ' active' : ''}"
        data-filter-status="${s}"
      >${label}</button>`;
    })
    .join('');

  return `
    <section class="filters">
      <div class="filter-group">
        <span class="filter-label">Type</span>
        <div class="filter-buttons">${typeButtons}</div>
      </div>
      <div class="filter-divider"></div>
      <div class="filter-group">
        <span class="filter-label">Status</span>
        <div class="filter-buttons">${statusButtons}</div>
      </div>
    </section>
  `;
}
