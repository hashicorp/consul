/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// Column definitions for the lock-sessions table. Each sortable column provides
// a `sortValue` comparator used by the generic Consul::DataTable; cell rendering
// itself lives in the template's :row block. Sorting is owned by these column
// headers (not a toolbar sort dropdown), matching the Figma design.
const COLUMNS = [
  {
    label: 'Name',
    sortKey: 'name',
    // Nameless sessions fall back to their ID in the cell, so sort by that too.
    sortValue: (item) => (item.Name || item.ID || '').toLowerCase(),
  },
  {
    label: 'Session ID',
  },
  {
    label: 'Lock delay',
    sortKey: 'lockDelay',
    sortValue: (item) => item.LockDelay || 0,
  },
  {
    label: 'Session TTL',
    sortKey: 'ttl',
    sortValue: (item) => item.TTL || '',
  },
  {
    label: 'Behaviour',
    sortKey: 'behavior',
    sortValue: (item) => (item.Behavior || '').toLowerCase(),
  },
  {
    label: 'Checks',
    width: '240px',
  },
  {
    label: 'Actions',
    align: 'right',
  },
];

/**
 * Consul::LockSession::Table
 *
 * Lock-sessions-tab specific configuration for the generic Consul::DataTable. It
 * supplies the concrete columns (and their sort comparators) and renders each
 * row's cells via the :row block, but owns no sorting / pagination state — that
 * all lives in the generic table.
 *
 * It does not perform any data fetching itself; it receives the already
 * fetched / filtered / searched `@items` from the data layer, plus an
 * `@ondelete` action for the per-row Invalidate control.
 *
 * The Invalidate control opens a single, table-level HDS Modal (the critical
 * "Invalidate Session?" confirmation) rather than a per-row inline dialog. The
 * pending session is held in `itemToDelete` while its modal is open, mirroring
 * the Consul::Peer::List delete-confirmation pattern.
 */
export default class ConsulLockSessionTable extends Component {
  columns = COLUMNS;

  // Holds the pending session while its invalidate-confirmation modal is open.
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
