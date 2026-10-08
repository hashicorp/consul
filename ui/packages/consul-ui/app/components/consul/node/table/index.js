/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Column definitions for the nodes table. Each sortable column provides a
// `sortValue` comparator used by the generic Consul::DataTable; cell rendering
// itself lives in the template's :row block.
const COLUMNS = [
  {
    label: 'Node name',
    sortKey: 'name',
    sortValue: (item) => (item.Node || '').toLowerCase(),
  },
  {
    label: 'Health',
    sortKey: 'health',
    sortValue: (item) => {
      const order = { critical: 0, warning: 1, passing: 2, empty: 3 };
      return order[item.Status] ?? 4;
    },
  },
  {
    label: 'Address',
    sortKey: 'address',
    sortValue: (item) => (item.Address || '').toLowerCase(),
  },
  {
    label: 'Version',
    sortKey: 'version',
    sortValue: (item) => (item.Version || '').toLowerCase(),
  },
  {
    label: 'Services',
    sortKey: 'services',
    sortValue: (item) => item.MeshServiceInstances.length,
  },
];

/**
 * Consul::Node::Table
 *
 * Nodes-index specific configuration for the generic Consul::DataTable. It
 * supplies the concrete columns (and their sort comparators) and renders each
 * row's cells via the :row block, but owns no sorting / pagination state — that
 * all lives in the generic table.
 *
 * It does not perform any data fetching itself; it receives the already
 * fetched / filtered / searched `@items` from the data layer.
 */
export default class ConsulNodeTable extends Component {
  columns = COLUMNS;

  // Health tooltip wording shown when hovering a node's health status cell.
  healthTooltip = (status) => {
    switch (status) {
      case 'critical':
        return 'At least one health check on this node is failing.';
      case 'warning':
        return 'At least one health check on this node has a warning.';
      case 'passing':
        return 'All health checks are passing.';
      default:
        return 'There are no health checks.';
    }
  };
}
