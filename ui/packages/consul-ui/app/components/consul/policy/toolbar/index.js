/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Policy "Type" filter options. Mirrors the legacy policy search bar kind filter
// and the `filter/predicates/policy` kind predicate values.
const KIND_OPTIONS = [
  { value: 'global-management', label: 'Global Management' },
  { value: 'standard', label: 'Standard' },
];

/**
 * Consul::Policy::Toolbar
 *
 * Policies-index specific configuration for the generic `Consul::ListToolbar`.
 * It supplies the Type filter group and the Datacenter filter group (built from
 * the `@dcs` datacenters loaded by the route before this toolbar renders); all
 * Filter Bar wiring, the free-text search and the "Search across" dropdown live
 * in the generic toolbar.
 */
export default class ConsulPolicyToolbar extends Component {
  get filterGroups() {
    return [
      {
        key: 'datacenter',
        text: 'Datacenters',
        options: (this.args.dcs || []).map((dc) => ({ value: dc.Name, label: dc.Name })),
      },
      { key: 'kind', text: 'Type', options: KIND_OPTIONS },
    ];
  }
}
