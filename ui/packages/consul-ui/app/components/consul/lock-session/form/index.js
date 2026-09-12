/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class ConsulLockSessionForm extends Component {
  // True while the invalidate confirmation modal is open.
  @tracked isConfirming = false;

  @action
  confirmInvalidate() {
    this.isConfirming = true;
  }

  @action
  cancelInvalidate() {
    this.isConfirming = false;
  }
}
