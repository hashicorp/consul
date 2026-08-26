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

// On peer detail pages a service's health can be "unknown" (e.g. a peered
// service with 0 instances, or a failing peer), so the Health filter offers an
// extra option there. Mirrors the legacy service search bar's `healthStates`.
const UNKNOWN_HEALTH_OPTION = { value: 'unknown', label: 'Unknown' };

// Quick-filter health buttons shown in the segmented control next to the
// Filter Bar. `value` maps to the same `status` filter values used by the
// "Health" group inside the Filter Bar, so both stay in sync.
const HEALTH_QUICK_FILTERS = [
  { value: 'passing', label: 'Healthy', icon: 'check-circle' },
  { value: 'warning', label: 'Warning', icon: 'alert-triangle' },
  { value: 'critical', label: 'Not-healthy', icon: 'x-circle' },
];

const KIND_OPTIONS = [
  { value: 'service', label: 'Service' },
  { value: 'api-gateway', label: 'API gateway' },
  { value: 'ingress-gateway', label: 'Ingress gateway' },
  { value: 'terminating-gateway', label: 'Terminating gateway' },
  { value: 'mesh-gateway', label: 'Mesh gateway' },
  { value: 'in-mesh', label: 'In service mesh' },
  { value: 'not-in-mesh', label: 'Not in service mesh' },
];

// The gateway "linked services" tab only ever offered a single "Registered /
// Not registered" filter (the `instance` predicate in
// app/filter/predicates/service.js) — it has no Health/Service type/External
// source data to filter on, so `@linked={{true}}` swaps in just this group
// instead of the full services-index filter set.
const INSTANCE_OPTIONS = [
  { value: 'registered', label: 'Registered' },
  { value: 'not-registered', label: 'Not Registered' },
];

// The linked-services tab has no health quick-filter buttons (see
// `filterGroups`), so its `:quickFilters` slot renders a grouped sort
// dropdown instead (Health status / Service name — mirrors
// Consul::HealthCheck::Toolbar's grouped sort control). Maps a
// `<Field>:<direction>` sort value to the translation key for its
// toggle-button label.
const SORT_LABEL_KEYS = {
  'Status:asc': 'common.sort.status.asc',
  'Status:desc': 'common.sort.status.desc',
  'Name:asc': 'common.sort.alpha.asc',
  'Name:desc': 'common.sort.alpha.desc',
};

/**
 * Consul::Service::Toolbar
 *
 * Services-index specific configuration for the generic `Consul::ListToolbar`.
 * It supplies the concrete filter groups (Health / Service type / External
 * source) and the health quick-filter buttons, but owns no Filter Bar wiring
 * itself — that all lives in the generic toolbar.
 *
 * @argument {boolean} [linked] - render the reduced filter set used by the
 *   gateway "linked services" tab (a single Instance/Registered filter, no
 *   health quick-filters) instead of the full services-index filter set, and
 *   a grouped sort dropdown (Health status / Service name) in its place.
 * @argument {object} [sort] - `{ value, change }` sort state, as built by the
 *   route template. Only used (and only rendered) in `@linked` mode.
 */
export default class ConsulServiceToolbar extends Component {
  @service intl;

  healthQuickFilters = HEALTH_QUICK_FILTERS;

  // Label shown on the linked-services sort dropdown's toggle button for the
  // currently active sort value.
  get sortLabel() {
    const key = SORT_LABEL_KEYS[this.args.sort?.value];
    return key ? this.intl.t(key) : '';
  }

  // Display label for an external-source value, using its brand name when one
  // exists (e.g. "kubernetes" -> "Kubernetes") and falling back to the raw
  // value otherwise. Mirrors the old service search bar's source labels.
  sourceLabel = (source) => {
    const key = `common.brand.${source}`;
    return this.intl.exists(key) ? this.intl.t(key) : source;
  };

  get sourceOptions() {
    // Prepend the synthetic "consul" source, which the `source` filter
    // predicate uses to match native services that have no external source.
    // This mirrors the old service search bar, which always offered it.
    const sources = ['consul', ...(this.args.sources || [])];
    return sources.map((source) => ({
      value: source,
      label: this.sourceLabel(source),
    }));
  }

  // The multi-select filter groups passed to the generic toolbar. External
  // source is only included when there are real external sources to choose
  // from (the synthetic "consul" option alone doesn't warrant the group).
  get filterGroups() {
    if (this.args.linked) {
      return [{ key: 'instance', text: 'Instance', options: INSTANCE_OPTIONS }];
    }

    // On peer detail pages services can be "unknown", so offer that extra
    // Health option (matching the old search bar's peer-aware health states).
    const healthOptions = this.args.peer
      ? [...HEALTH_OPTIONS, UNKNOWN_HEALTH_OPTION]
      : HEALTH_OPTIONS;
    const groups = [
      { key: 'status', text: 'Health', options: healthOptions },
      { key: 'kind', text: 'Service type', options: KIND_OPTIONS },
    ];
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
