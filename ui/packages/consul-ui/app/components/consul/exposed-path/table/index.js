/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Column definitions for the service-instance Exposed Paths table. Address
// has no sort affordance (matches the Figma design — it's a derived/combined
// value, not a raw field); the other four columns are sortable.
const COLUMNS = [
  {
    label: 'Address',
  },
  {
    label: 'Protocol',
    sortKey: 'protocol',
    sortValue: (item) => (item.Protocol || '').toLowerCase(),
  },
  {
    label: 'Listener port',
    sortKey: 'listenerPort',
    sortValue: (item) => item.ListenerPort,
  },
  {
    // Figma's header just says "Local path", but the value is a port number
    // (LocalPathPort), not a path — that reads as ambiguous next to the real
    // "Path" column, so the fuller label is kept here.
    label: 'Local path port',
    sortKey: 'localPath',
    sortValue: (item) => item.LocalPathPort,
  },
  {
    label: 'Path',
    sortKey: 'path',
    sortValue: (item) => (item.Path || '').toLowerCase(),
  },
];

/**
 * Consul::ExposedPath::Table
 *
 * Service-instance Exposed Paths-tab specific configuration for the generic
 * Consul::DataTable. Supplies the concrete columns (and their sort
 * comparators) and renders each row's cells via the :row block; owns no
 * sorting/pagination state itself — that lives in the generic table.
 *
 * @argument {Array} items - the exposed paths to display.
 * @argument {string} address - the service/node address the exposed paths
 *   are reachable through, used to build each row's combined address.
 */
export default class ConsulExposedPathTable extends Component {
  columns = COLUMNS;

  // Mirrors the combined-address logic the old Consul::ExposedPath::List
  // used: `address:listenerPort` followed directly by the path (which
  // already includes its own leading slash).
  combinedAddressFor = (item) => {
    return `${this.args.address}:${item.ListenerPort}${item.Path}`;
  };
}
