/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

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
    label: 'Address',
  },
];

/**
 * Consul::Upstream::Table
 *
 * Upstreams-tab (ingress gateway) specific configuration for the generic
 * Consul::DataTable. It supplies the concrete columns (and their sort comparators)
 * and renders each row's cells via the :row block; owns no sorting / pagination
 * state itself — that all lives in the generic table.
 *
 * @argument {Array} items - the upstream services to display.
 * @argument {string} [dc] - current datacenter slug.
 * @argument {string} [nspace] - current namespace.
 * @argument {string} [partition] - current partition.
 * @argument {boolean} [card] - whether to wrap the table in card border styling.
 */
export default class ConsulUpstreamTable extends Component {
  columns = COLUMNS;

  // Mirrors the link param logic from Consul::Service::Table so that
  // cross-partition / cross-namespace links keep working.
  linkParams = (item) => {
    const params = {};
    if (this.args.partition && item.Partition && item.Partition !== this.args.partition) {
      params.partition = item.Partition;
      params.nspace = item.Namespace;
    } else if (this.args.nspace && item.Namespace && item.Namespace !== this.args.nspace) {
      params.nspace = item.Namespace;
    }
    return params;
  };
}
