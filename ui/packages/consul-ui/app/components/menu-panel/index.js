/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { schedule } from '@ember/runloop';

export default class MenuPanelComponent extends Component {
  @service dom;

  @tracked isConfirmation = false;

  @action
  connect(element) {
    schedule('afterRender', () => {
      // if theres only a single choice in the menu and it doesn't have an
      // immediate button/link/label to click then it will be a
      // confirmation/informed action
      const isConfirmationMenu = this.dom.element(
        'li:only-child > [role="menu"]:first-child',
        element
      );
      this.isConfirmation = typeof isConfirmationMenu !== 'undefined';
    });
  }
}
