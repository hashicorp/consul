/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// Column definitions for the admin partitions table.
// The Name column is sortable; Description and Actions are not.
const COLUMNS = [
  {
    label: 'Name',
    sortKey: 'name',
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
 * row's cells via the :row block. Owns no sorting / pagination state — that
 * all lives in the generic table. Receives already-filtered/searched @items
 * from the data layer and delegates row deletion to @ondelete.
 */
export default class ConsulPartitionListTable extends Component {
  columns = COLUMNS;

  // The HDS Dropdown's Delete item opens a confirmation modal rather than
  // deleting immediately; itemToDelete holds the pending partition while the
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
      this.args.ondelete(item);
    }
  }
}
