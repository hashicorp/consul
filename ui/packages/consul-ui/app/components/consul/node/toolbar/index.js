/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

const HEALTH_OPTIONS = [
  { value: 'passing', label: 'Passing' },
  { value: 'warning', label: 'Warning' },
  { value: 'critical', label: 'Critical' },
];

// Quick-filter health buttons shown in the segmented control next to the
// Filter Bar. `value` maps to the same `status` filter values used by the
// "Health" group inside the Filter Bar, so both stay in sync.
const HEALTH_QUICK_FILTERS = [
  { value: 'passing', label: 'Passing', icon: 'check-circle-fill' },
  { value: 'warning', label: 'Warning', icon: 'alert-triangle-fill' },
  { value: 'critical', label: 'Critical', icon: 'x-circle-fill' },
];

/**
 * Consul::Node::Toolbar
 *
 * Nodes-index specific configuration for the generic `Consul::ListToolbar`. It
 * supplies the concrete filter groups (Health / Version) and the health
 * quick-filter buttons, but owns no Filter Bar wiring itself — that all lives
 * in the generic toolbar.
 */
export default class ConsulNodeToolbar extends Component {
  healthQuickFilters = HEALTH_QUICK_FILTERS;

  // The Consul versions available for filtering come from the API as a list of
  // raw version strings; display them as "<version>.x" to match the old UI.
  get versionOptions() {
    return (this.args.versions || []).map((version) => ({
      value: version,
      label: `${version}.x`,
    }));
  }

  // The multi-select filter groups passed to the generic toolbar. The version
  // group is only included when the API reports more than one version.
  get filterGroups() {
    const groups = [{ key: 'status', text: 'Health', options: HEALTH_OPTIONS }];
    if (this.versionOptions.length) {
      groups.push({
        key: 'version',
        text: 'Version',
        searchEnabled: true,
        options: this.versionOptions,
      });
    }
    return groups;
  }
}
