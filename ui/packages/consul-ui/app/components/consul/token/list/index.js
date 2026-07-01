/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// Column definitions for the tokens table. Sorting and searching continue to be
// driven by the toolbar SearchBar, so columns are non-sortable here; cell
// rendering lives in the template's :row block. The trailing "Actions" column
// is right-aligned.
const COLUMNS = [
  { label: 'Name' },
  { label: 'Scope' },
  { label: 'Description' },
  { label: 'Secret' },
  { label: 'Actions', align: 'right' },
];
// Secret column shows the SecretID copy link; trailing Actions column is right-aligned.

/**
 * Consul::Token::List
 *
 * Tokens specific configuration for the generic Consul::DataTable. It supplies
 * the columns and renders each row's cells via the :row block. It owns no
 * sorting / pagination / data fetching; it receives already
 * fetched/filtered/searched `@items` and delegates use/logout/delete/clone to
 * the handlers passed in. The use / logout / delete actions open a confirmation
 * modal before invoking their handler.
 */
export default class ConsulTokenList extends Component {
  columns = COLUMNS;

  // Holds the pending token + the action type ('use' | 'logout' | 'delete')
  // while its confirmation modal is open.
  @tracked itemToConfirm = null;
  @tracked confirmType = null;

  @action
  confirm(type, item) {
    this.confirmType = type;
    this.itemToConfirm = item;
  }

  @action
  cancelConfirm() {
    this.confirmType = null;
    this.itemToConfirm = null;
  }

  @action
  invokeConfirm() {
    const item = this.itemToConfirm;
    const type = this.confirmType;
    this.itemToConfirm = null;
    this.confirmType = null;
    if (!item) {
      return;
    }
    switch (type) {
      case 'use':
        this.args.onuse(item);
        break;
      case 'logout':
        this.args.onlogout(item);
        break;
      case 'delete':
        this.args.ondelete(item);
        break;
    }
  }
}
