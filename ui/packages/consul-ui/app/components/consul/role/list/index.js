/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// Column definitions for the roles table. Sorting and searching continue to be
// driven by the toolbar SearchBar, so columns are non-sortable here; cell
// rendering lives in the template's :row block. The trailing "Actions" column
// is right-aligned.
const COLUMNS = [
  { label: 'Role name' },
  { label: 'Description' },
  { label: 'Actions', align: 'right' },
];

/**
 * Consul::Role::List
 *
 * Roles specific configuration for the generic Consul::DataTable. It supplies
 * the columns and renders each row's cells via the :row block. It owns no
 * sorting / pagination / data fetching; it receives already
 * fetched/filtered/searched `@items` and delegates delete to the handler passed
 * in. The delete action opens a confirmation modal before invoking its handler.
 */
export default class ConsulRoleList extends Component {
  columns = COLUMNS;

  // Holds the pending role while its delete confirmation modal is open.
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
