/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Visual treatment per health status. `empty` has no entry because it renders
// as plain text rather than a badge (see the template); anything unrecognised
// falls back to `unknown`.
const PRESENTATION = {
  passing: { label: 'Passing', icon: 'check', color: 'success', type: 'filled' },
  warning: { label: 'Warning', icon: 'alert-triangle', color: 'warning', type: 'filled' },
  critical: { label: 'Critical', icon: 'x', color: 'critical', type: 'filled' },
  unknown: { label: 'Unknown', icon: 'help', color: 'neutral', type: 'outlined' },
};

/**
 * Consul::HealthBadge
 *
 * The single health badge used by the services, service-instances and
 * upstreams tables, and for the overall badge next to a service title.
 *
 * It renders two different wordings, because the designs use different
 * language depending on whether counts are shown:
 *
 *  - default — the status on its own ("Passing"), used where a row represents
 *    a whole service and individual check counts would be noise.
 *  - `@showCount` — describes the checks ("All checks passing",
 *    "3/6 checks failing"), used on the service-instances table where a row is
 *    a single instance and the counts are meaningful.
 *
 * `empty` (no checks registered) is deliberately *not* a badge in either mode —
 * the designs show it as plain text, since there is no health to report.
 *
 * @argument {string} status - `passing` | `warning` | `critical` | `empty`;
 *   anything else renders as `unknown`.
 * @argument {boolean} [showCount] - use the check-describing wording above.
 * @argument {number} [count] - checks in the displayed status.
 * @argument {number} [total] - total checks.
 * @argument {number} [passing] - checks that are passing; lets the badge say
 *   "All checks failing" when none are.
 * @argument {string} [emptyText] - text for the `empty` state, e.g.
 *   "No service checks" / "No node checks".
 * @argument {string} [tooltip] - tooltip text; defaults to the badge label.
 */
export default class ConsulHealthBadge extends Component {
  get isEmpty() {
    return this.args.status === 'empty';
  }

  get emptyText() {
    return this.args.emptyText ?? 'No checks';
  }

  get presentation() {
    return PRESENTATION[this.args.status] ?? PRESENTATION.unknown;
  }

  get text() {
    if (!this.args.showCount) {
      return this.presentation.label;
    }
    return this.countText ?? this.presentation.label;
  }

  // Check-describing wording, used only when @showCount is set. Returns
  // undefined for statuses where a count says nothing useful (e.g. unknown).
  get countText() {
    const { status, count, total, passing } = this.args;

    if (status === 'passing') {
      return 'All checks passing';
    }

    if (status === 'warning' || status === 'critical') {
      // "all failing" covers warning as well as critical, matching the
      // wording used before the HDS migration.
      if (passing === 0) {
        return 'All checks failing';
      }
      if (total != null) {
        return `${count ?? 0}/${total} checks failing`;
      }
    }

    return undefined;
  }
}
