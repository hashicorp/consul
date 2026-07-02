/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

const HEALTH_OPTIONS = [
  { value: 'passing', label: 'Healthy' },
  { value: 'warning', label: 'Warning' },
  { value: 'critical', label: 'Not-healthy' },
  { value: 'empty', label: 'No health checks' },
];

// Quick-filter health buttons shown in the segmented control next to the
// Filter Bar. `value` maps to the same `status` filter values used by the
// "Health" group inside the Filter Bar, so both stay in sync.
const HEALTH_QUICK_FILTERS = [
  { value: 'passing', label: 'Healthy', icon: 'check-circle-fill' },
  { value: 'warning', label: 'Warning', icon: 'alert-triangle-fill' },
  { value: 'critical', label: 'Not-healthy', icon: 'x-circle-fill' },
];

/**
 * Consul::ServiceInstance::Toolbar
 *
 * Service-instances specific configuration for the generic
 * `Consul::ListToolbar`. It supplies the concrete filter groups (Health /
 * External source) and the health quick-filter buttons, but owns no Filter Bar
 * wiring itself — that all lives in the generic toolbar. Mirrors
 * `Consul::Service::Toolbar` minus the service-type group, which doesn't apply
 * to a single service's instances.
 */
export default class ConsulServiceInstanceToolbar extends Component {
  @service intl;

  healthQuickFilters = HEALTH_QUICK_FILTERS;

  // Display label for an external-source value, using its brand name when one
  // exists (e.g. "kubernetes" -> "Kubernetes") and falling back to the raw
  // value otherwise. Mirrors the old service-instance search bar's labels.
  sourceLabel = (source) => {
    const key = `common.brand.${source}`;
    return this.intl.exists(key) ? this.intl.t(key) : source;
  };

  get sourceOptions() {
    return (this.args.sources || []).map((source) => ({
      value: source,
      label: this.sourceLabel(source),
    }));
  }

  // The multi-select filter groups passed to the generic toolbar. External
  // source is only included when there are real external sources to choose
  // from.
  get filterGroups() {
    const groups = [{ key: 'status', text: 'Health', options: HEALTH_OPTIONS }];
    if ((this.args.sources || []).length) {
      groups.push({
        key: 'source',
        text: 'External source',
        searchEnabled: true,
        options: this.sourceOptions,
      });
    }
    return groups;
  }
}
