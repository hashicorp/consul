/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// Column definitions for the admin partitions table.
// The Name column is sortable; Description and Actions are not.
//
// `sortField` is the field name used by the toolbar's Sort dropdown / the
// data-layer `sort` query param (see templates/dc/partitions/index.hbs,
// e.g. "Name:asc"), which is capitalized differently from `sortKey` (this
// table's own internal column identifier). It's what lets this table and
// Consul::Partition::Toolbar's dropdown translate between each other and
// share the same underlying sort state.
const COLUMNS = [
  {
    label: 'Name',
    sortKey: 'name',
    sortField: 'Name',
    sortValue: (item) => (item.Name || '').toLowerCase(),
  },
  {
    label: 'Description',
  },
  {
    label: 'Actions',
    align: 'right',
  },
];

/**
 * Consul::Partition::List::Table
 *
 * Admin-partitions-index specific configuration for the generic
 * Consul::DataTable. Supplies the concrete column definitions and renders each
 * row's cells via the :row block. Owns no sorting / pagination state itself —
 * pagination lives in the generic table, and sorting is driven by `@sort`
 * (the same sort state templates/dc/partitions/index.hbs hands to
 * Consul::Partition::Toolbar's Sort dropdown), passed through to
 * Consul::DataTable in "controlled" mode so a column-header click and a
 * toolbar selection both read/write the one shared sort state instead of
 * silently overriding one another. Receives already-filtered/searched @items
 * from the data layer and delegates row deletion to @ondelete.
 */
export default class ConsulPartitionListTable extends Component {
  columns = COLUMNS;

  // The HDS Dropdown's Delete item opens a confirmation modal rather than
  // deleting immediately; itemToDelete holds the pending partition while the
  // modal is open.
  @tracked itemToDelete = null;

  get sortBy() {
    const [field] = (this.args.sort?.value || '').split(':');
    return this.columns.find((column) => column.sortField === field)?.sortKey;
  }

  get sortOrder() {
    const [, order] = (this.args.sort?.value || '').split(':');
    return order;
  }

  @action
  onSort(sortKey, sortOrder) {
    const column = this.columns.find((column) => column.sortKey === sortKey);
    if (column?.sortField) {
      this.args.sort?.change({ target: { selected: `${column.sortField}:${sortOrder}` } });
    }
  }

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
      this.args.ondelete(item);
    }
  }
}
