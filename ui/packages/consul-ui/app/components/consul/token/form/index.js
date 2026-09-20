/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

/**
 * Consul::Token::Form
 *
 * Owns the Delete confirmation modal state.  All other actions (Save, Cancel,
 * Delete invoke) are delegated to the parent via @onCreate / @onUpdate /
 * @onCancel / @onDelete args.
 */
export default class ConsulTokenForm extends Component {
  // true while the delete-confirmation modal is open
  @tracked showDeleteModal = false;

  @action
  openDeleteModal() {
    this.showDeleteModal = true;
  }

  @action
  closeDeleteModal() {
    this.showDeleteModal = false;
  }

  @action
  confirmDelete() {
    this.showDeleteModal = false;
    this.args.onDelete(this.args.item);
  }
}
