/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Shared vocabulary for the Health filters rendered by the Nodes, Services and
 * Service-instances toolbars.
 *
 * Each of those three toolbars used to hardcode its own English labels, which
 * is precisely how they drifted apart: Services and Service-instances showed
 * "Healthy"/"Not-healthy" while Nodes showed "Passing"/"Critical" for the very
 * same `status` filter values. Labels now come from the shared
 * `common.consul.*` translations, which `Consul::HealthCheck::Toolbar` already
 * uses, so there is a single place to change the wording.
 */

// Icons for the quick-filter segmented control. Only the three "real" statuses
// get a quick filter; `empty` and `unknown` are Filter Bar options only.
const QUICK_FILTER_ICONS = {
  passing: 'check-circle',
  warning: 'alert-triangle',
  critical: 'x-circle',
};

const QUICK_FILTER_STATUSES = ['passing', 'warning', 'critical'];

/**
 * Build Filter Bar options for the given `status` values.
 *
 * @param {object} intl - the `intl` service
 * @param {string[]} statuses - `status` filter values, in display order
 */
export function healthOptions(intl, statuses) {
  return statuses.map((value) => ({
    value,
    label: intl.t(`common.consul.${value}`),
  }));
}

/**
 * Build the health quick-filter buttons for the segmented control next to the
 * Filter Bar. `value` maps to the same `status` filter values used by the
 * "Health" group inside the Filter Bar, so both stay in sync.
 *
 * @param {object} intl - the `intl` service
 */
export function healthQuickFilters(intl) {
  return QUICK_FILTER_STATUSES.map((value) => ({
    value,
    label: intl.t(`common.consul.${value}`),
    icon: QUICK_FILTER_ICONS[value],
  }));
}
