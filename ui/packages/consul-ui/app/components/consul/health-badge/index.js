/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Visual treatment per health status. `passing` / `warning` / `critical` are
// the three real statuses and use the same wording as the health filters and
// the health check list; `empty` means no checks are registered, and anything
// else (e.g. a peered service whose health can't be determined) falls back to
// `unknown`.
const PRESENTATION = {
  passing: { label: 'Passing', icon: 'check', color: 'success', type: 'filled' },
  warning: { label: 'Warning', icon: 'alert-triangle', color: 'warning', type: 'filled' },
  critical: { label: 'Critical', icon: 'x', color: 'critical', type: 'filled' },
  empty: { label: 'No checks', color: 'neutral', type: 'outlined' },
  unknown: { label: 'Unknown', icon: 'help', color: 'neutral', type: 'outlined' },
};

// A count only says something meaningful for the three real statuses — there
// is nothing to count for "no checks" or "unknown".
const COUNTABLE = ['passing', 'warning', 'critical'];

/**
 * Consul::HealthBadge
 *
 * The single health badge used by the services, service-instances and
 * upstreams tables. Previously each of those inlined its own ~60-line
 * `Hds::Badge` conditional, which is how their wording drifted apart.
 *
 * @argument {string} status - `passing` | `warning` | `critical` | `empty`;
 *   anything else renders as `unknown`.
 * @argument {boolean} [showCount] - append `<count>/<total>` to the label.
 *   The services and upstreams lists show the status on its own; the
 *   service-instances list shows it with counts.
 * @argument {number} [count] - checks in the displayed status.
 * @argument {number} [total] - total checks.
 * @argument {string} [tooltip] - tooltip text; defaults to the badge label.
 */
export default class ConsulHealthBadge extends Component {
  get presentation() {
    return PRESENTATION[this.args.status] ?? PRESENTATION.unknown;
  }

  get text() {
    const { label } = this.presentation;
    const { showCount, status, count, total } = this.args;
    if (!showCount || !COUNTABLE.includes(status) || total == null) {
      return label;
    }
    return `${count ?? 0}/${total} ${label}`;
  }
}
