/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

import {
  healthOptions as buildHealthOptions,
  healthQuickFilters as buildHealthQuickFilters,
} from 'consul-ui/utils/health-filter-options';

const HEALTH_STATUSES = ['passing', 'warning', 'critical'];

/**
 * Consul::Node::Toolbar
 *
 * Nodes-index specific configuration for the generic `Consul::ListToolbar`. It
 * supplies the concrete filter groups (Health / Version) and the health
 * quick-filter buttons, but owns no Filter Bar wiring itself — that all lives
 * in the generic toolbar.
 */
export default class ConsulNodeToolbar extends Component {
  @service intl;

  get healthQuickFilters() {
    return buildHealthQuickFilters(this.intl);
  }

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
    const groups = [
      {
        key: 'status',
        text: 'Health',
        options: buildHealthOptions(this.intl, HEALTH_STATUSES),
      },
    ];
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
