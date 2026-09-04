/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// Column definitions for the intentions table. Each sortable column provides a
// `sortValue` comparator used by the generic Consul::DataTable; cell rendering
// itself lives in the template's :row block. The trailing "Actions" column is
// intentionally non-sortable.
const COLUMNS = [
  {
    label: 'Source',
    sortKey: 'source',
    sortValue: (item) => (item.SourceName || '').toLowerCase(),
  },
  {
    label: 'Intention type',
    sortKey: 'action',
    sortValue: (item) => (item.Action || 'app-aware').toLowerCase(),
  },
  {
    label: 'Destination',
    sortKey: 'destination',
    sortValue: (item) => (item.DestinationName || '').toLowerCase(),
  },
  {
    label: 'Permissions',
    sortKey: 'permissions',
    sortValue: (item) => item.Permissions.length,
    tooltip:
      "Permissions intercept an Intention's traffic using Layer 7 criteria, such as path prefixes and http headers.",
  },
  {
    label: 'Status',
    sortKey: 'status',
    sortValue: (item) => (item.IsManagedByCRD ? 0 : 1),
  },
  {
    label: 'Actions',
    align: 'right',
  },
];

/**
 * Consul::Intention::List::Table
 *
 * Intentions specific configuration for the generic Consul::DataTable. It
 * supplies the concrete columns (and their sort comparators) and renders each
 * row's cells via the :row block, but owns no sorting / pagination state — that
 * all lives in the generic table. It performs no data fetching itself; it
 * receives the already fetched / filtered / searched `@items` from the data
 * layer, and delegates row deletion to `@delete`.
 */
export default class ConsulIntentionListTable extends Component {
  columns = COLUMNS;

  // The HDS Dropdown's Delete item opens a confirmation modal rather than
  // deleting immediately; `itemToDelete` holds the pending intention while the
  // modal is open.
  @tracked itemToDelete = null;

  @action
  confirmDelete(item) {
    this.itemToDelete = item;
  }

  @action
  cancelDelete() {
    this.itemToDelete = null;
  }

  @action
  invokeDelete() {
    const item = this.itemToDelete;
    this.itemToDelete = null;
    if (item) {
      this.args.delete(item);
    }
  }
}
