/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action, set } from '@ember/object';

export default class NspaceForm extends Component {
  @tracked isConfirmingDelete = false;

  @action confirmDelete() {
    this.isConfirmingDelete = true;
  }

  @action cancelDelete() {
    this.isConfirmingDelete = false;
  }

  @action onSubmit(item) {
    const onSubmit = this.args.onsubmit;
    if (onSubmit) return onSubmit(item);
  }

  @action onDelete(item) {
    const { onsubmit, ondelete } = this.args;

    if (ondelete) {
      return ondelete(item);
    } else {
      if (onsubmit) {
        return onsubmit(item);
      }
    }
  }

  @action updateDescription(item, event) {
    set(item, 'Description', event.target.value);
  }

  @action onCancel(item) {
    const { oncancel, onsubmit } = this.args;

    if (oncancel) {
      return oncancel(item);
    } else {
      if (onsubmit) {
        return onsubmit(item);
      }
    }
  }
}
