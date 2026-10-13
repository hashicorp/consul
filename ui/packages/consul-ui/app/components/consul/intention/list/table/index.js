/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// Column definitions for the intentions table. Per the migration designs,
// only Source and Permissions are sortable columns; Intention type,
// Destination, and Status have no `sortKey`, so Consul::DataTable renders a
// plain (non-interactive) header for them. The trailing "Actions" column is
// intentionally non-sortable too.
//
// The Permissions column's explanatory header tooltip is added conditionally
// (see the `columns` getter below) rather than baked into this static list,
// so it only appears when at least one row actually has permissions to
// explain.
const PERMISSIONS_TOOLTIP =
  "Permissions intercept an Intention's traffic using Layer 7 criteria, such as path prefixes and http headers.";

const COLUMNS = [
  {
    label: 'Source',
    sortKey: 'source',
    sortValue: (item) => (item.SourceName || '').toLowerCase(),
  },
  {
    label: 'Intention type',
  },
  {
    label: 'Destination',
  },
  {
    label: 'Permissions',
    sortKey: 'permissions',
    sortValue: (item) => item.Permissions.length,
  },
  {
    label: 'Status',
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
  // Only show the Permissions header tooltip when at least one row in the
  // current view actually has permissions — otherwise there's nothing for it
  // to explain.
  get columns() {
    const hasPermissions = (this.args.items || []).some(
      (item) => (item.Permissions?.length || 0) > 0
    );
    return COLUMNS.map((column) =>
      column.label === 'Permissions' && hasPermissions
        ? { ...column, tooltip: PERMISSIONS_TOOLTIP }
        : column
    );
  }

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
