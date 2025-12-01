/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { schedule } from '@ember/runloop';
import { set } from '@ember/object';

import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  isConfirmation: false,

  actions: {
    connect: function ($el) {
      schedule('afterRender', () => {
        if (!this.isDestroyed) {
          // if theres only a single choice in the menu and it doesn't have an
          // immediate button/link/label to click then it will be a
          // confirmation/informed action
          const isConfirmationMenu = this.dom.element(
            'li:only-child > [role="menu"]:first-child',
            $el
          );
          set(this, 'isConfirmation', typeof isConfirmationMenu !== 'undefined');
        }
      });
    },
    change: function (e) {
      // not being used
    },
  },
});
