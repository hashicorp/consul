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
const COLUMNS = [
  { label: 'Peer name', sortKey: 'name', sortValue: (item) => (item.Name || '').toLowerCase() },
  {
    label: 'Status',
    sortKey: 'state',
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
