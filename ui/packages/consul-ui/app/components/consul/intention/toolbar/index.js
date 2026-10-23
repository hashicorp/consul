/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Intention "type" (a.k.a. access) filter options. The synthetic "app-aware"
// value matches intentions with no Action (Layer 7 / permission based), exactly
// as the legacy intention search bar's access filter did. Mirrors the
// `filter/predicates/intention` access predicate values.
const ACCESS_OPTIONS = [
  { value: 'allow', label: 'Allow' },
  { value: 'deny', label: 'Deny' },
  { value: 'app-aware', label: 'App aware' },
];

// Quick-filter buttons shown in the segmented control next to the Filter Bar.
// `value` maps to the same `access` filter values used by the "Intention type"
// group inside the Filter Bar, so both stay in sync.
const ACCESS_QUICK_FILTERS = [
  { value: 'allow', label: 'Allow', icon: 'check' },
  { value: 'deny', label: 'Deny', icon: 'x' },
  { value: 'app-aware', label: 'App aware', icon: 'info' },
];

/**
 * Consul::Intention::Toolbar
 *
 * Intentions-index specific configuration for the generic `Consul::ListToolbar`.
 * It supplies the concrete filter group (Intention type) and the quick-filter
 * buttons, but owns no Filter Bar wiring itself — that all lives in the generic
 * toolbar.
 */
export default class ConsulIntentionToolbar extends Component {
  accessQuickFilters = ACCESS_QUICK_FILTERS;

  get filterGroups() {
    return [{ key: 'access', text: 'Intention type', options: ACCESS_OPTIONS }];
  }
}
