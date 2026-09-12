/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// Cancel button shared by the peer create form's tabs. When the form has
// unsaved input, clicking it asks for confirmation before discarding it and
// invoking `@onconfirm` (navigating back to the peers list); an empty form
// can be left without asking.
export default class ConsulPeerFormCancelComponent extends Component {
  @tracked confirming = false;

  get dirty() {
    return Boolean(this.args.dirty);
  }

  @action
  click() {
    if (this.dirty) {
      this.confirming = true;
    } else {
      this.args.onconfirm();
    }
  }

  @action
  confirm() {
    this.confirming = false;
    this.args.onconfirm();
  }

  @action
  keepEditing() {
    this.confirming = false;
  }
}
