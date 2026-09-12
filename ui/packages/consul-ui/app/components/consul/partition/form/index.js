/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class ConsulPartitionForm extends Component {
  // Holds the pending item while the delete-confirmation modal is open.
  @tracked isConfirmingDelete = false;

  @action
  confirmDelete() {
    this.isConfirmingDelete = true;
  }

  @action
  cancelDelete() {
    this.isConfirmingDelete = false;
  }
}
