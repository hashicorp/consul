/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class ConsulLockSessionNotificationsComponent extends Component {
  @tracked isDismissed = false;

  @action
  dismiss() {
    this.isDismissed = true;
  }
}
