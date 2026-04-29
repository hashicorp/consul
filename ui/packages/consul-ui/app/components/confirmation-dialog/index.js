/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class ConfirmationDialogComponent extends Component {
  tagName = '';
  message = 'Are you sure?';
  @tracked confirming = false;
  permanent = false;

  @action
  cancel() {
    this.confirming = false;
  }

  @action
  execute() {
    this.confirming = false;
  }

  @action
  confirm() {
    this.confirming = true;
  }
}
