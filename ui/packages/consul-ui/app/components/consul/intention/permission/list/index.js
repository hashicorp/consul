/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

/**
 * Consul::Intention::Permission::List
 *
 * Renders the list of L7 permission cards for an intention's create/edit page.
 * The Delete button opens a confirmation modal rather than deleting immediately;
 * `itemToDelete` holds the pending permission while the modal is open.
 */
export default class ConsulIntentionPermissionList extends Component {
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
      // optional() is not needed here — args.ondelete is passed by the parent
      if (typeof this.args.ondelete === 'function') {
        this.args.ondelete(item);
      }
    }
  }
}
