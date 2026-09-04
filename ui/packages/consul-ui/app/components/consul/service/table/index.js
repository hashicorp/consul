/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Column definitions for the services table. Each sortable column provides a
// `sortValue` comparator used by the generic Consul::DataTable; cell rendering
// itself lives in the template's :row block.
const COLUMNS = [
  {
    label: 'Service name',
    sortKey: 'name',
    sortValue: (item) => (item.Name || '').toLowerCase(),
  },
  {
    label: 'Health',
    sortKey: 'health',
    sortValue: (item) => {
      const order = { critical: 0, warning: 1, passing: 2, empty: 3, unknown: 4 };
      return order[item.MeshStatus] ?? 5;
    },
  },
  {
    label: 'Service mesh',
    sortKey: 'mesh',
    sortValue: (item) => {
      if (item.ConnectedWithProxy && item.ConnectedWithGateway) return 0;
      if (item.ConnectedWithProxy) return 1;
      if (item.ConnectedWithGateway) return 2;
      return 3;
    },
  },
  {
    label: 'Service type',
    sortKey: 'type',
    sortValue: (item) => (item.Kind || '').toLowerCase(),
  },
  {
    label: 'External Source',
    sortKey: 'source',
    sortValue: (item) => (item.ExternalSources || []).join(',').toLowerCase(),
  },
];

/**
 * Consul::Service::Table
 *
 * Services-index specific configuration for the generic Consul::DataTable. It
 * supplies the concrete columns (and their sort comparators) and renders each
 * row's cells via the :row block, but owns no sorting / pagination state — that
 * all lives in the generic table.
 *
 * It does not perform any data fetching itself; it receives the already
 * fetched / filtered / searched `@items` from the data layer.
 *
 * @argument {boolean} [showMesh] - whether to render the "Service mesh"
 *   column. Defaults to true. The gateway "linked services" tab's endpoint
 *   (`/gateways/for-service`) never returns `ConnectedWithProxy` /
 *   `ConnectedWithGateway`, so that column can never have real data there —
 *   callers on that tab pass `@showMesh={{false}}` to omit it entirely rather
 *   than show a column that's always empty.
 */
export default class ConsulServiceTable extends Component {
  // Whether the "Service mesh" column is included, both in the header and in
  // each row. Filtering it out of `columns` (rather than just hiding the cell)
  // keeps the header/row cell counts in sync.
  get showMesh() {
    return this.args.showMesh !== false;
  }

  get columns() {
    if (this.showMesh) {
      return COLUMNS;
    }
    return COLUMNS.filter((column) => column.sortKey !== 'mesh');
  }

  // Mirrors the link param logic from Consul::Service::List so that
  // cross-partition / cross-namespace / peered links keep working.
  linkParams = (item) => {
    const hash = {};

    if (item.Partition && this.args.partition !== item.Partition) {
      hash.partition = item.Partition;
      hash.nspace = this.args.Namespace;
    } else if (item.Namespace && this.args.nspace !== item.Namespace) {
      hash.nspace = item.Namespace;
    }

    if (item.PeerName) {
      hash.peer = item.PeerName;
    }

    return hash;
  };
}
