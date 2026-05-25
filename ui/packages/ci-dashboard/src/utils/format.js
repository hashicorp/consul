// Shared utilities used by all components.

const HTML_ESCAPE_MAP = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' };

export function escapeHtml(str) {
  return String(str ?? '').replace(/[&<>"]/g, (c) => HTML_ESCAPE_MAP[c]);
}

export function statusBadge(status) {
  const classes = {
    passed: 'badge-passed',
    failed: 'badge-failed',
    skipped: 'badge-skipped',
  };
  const cls = classes[status] ?? 'badge-unknown';
  return `<span class="status-badge ${cls}">${escapeHtml(status)}</span>`;
}

export function typeBadge(type) {
  const cls = `type-${escapeHtml(String(type).toLowerCase())}`;
  return `<span class="type-badge ${cls}">${escapeHtml(type)}</span>`;
}

export function formatTimestamp(iso) {
  try {
    return new Date(iso).toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return iso;
  }
}
