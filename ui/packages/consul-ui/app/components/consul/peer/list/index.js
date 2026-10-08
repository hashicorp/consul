/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { schema } from 'consul-ui/models/peer';

// Status sorts in declared order (Pending -> Deleting), matching
// app/sort/comparators/peer.js.
//
// `sortField` is the field name used by the toolbar's Sort dropdown / the
// data-layer `sort` query param (see templates/dc/peers/index.hbs, e.g.
// "Name:asc"/"State:asc"), which is capitalized differently from `sortKey`
// (this table's own internal column identifier). It's what lets this table
// and Consul::Peer::Toolbar's dropdown translate between each other and
// share the same underlying sort state.
const COLUMNS = [
  {
    label: 'Peer name',
    sortKey: 'name',
    sortField: 'Name',
    sortValue: (item) => (item.Name || '').toLowerCase(),
  },
  {
    label: 'Status',
    sortKey: 'state',
    sortField: 'State',
    sortValue: (item) => schema.State.allowedValues.indexOf(item.State),
  },
  { label: 'Imported services count' },
  { label: 'Exported services count' },
  { label: 'Actions', align: 'right' },
];

export default class ConsulPeerList extends Component {
  columns = COLUMNS;

  // Holds the pending peer while its delete confirmation modal is open.
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
