/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { set } from '@ember/object';
import Slotted from 'block-slots';
import A11yDialog from 'a11y-dialog';
import { schedule } from '@ember/runloop';

export default Component.extend(Slotted, {
  tagName: '',
  onclose: function () {},
  onopen: function () {},
  isOpen: false,
  actions: {
    connect: function ($el) {
      this.dialog = new A11yDialog($el);
      this.dialog.on('hide', () => {
        schedule('afterRender', (_) => set(this, 'isOpen', false));
        this.onclose({ target: $el });
      });
      this.dialog.on('show', () => {
        set(this, 'isOpen', true);
        this.onopen({ target: $el });
      });
      if (this.open) {
        this.actions.open.apply(this, []);
      }
    },
    disconnect: function ($el) {
      this.dialog.destroy();
    },
    open: function () {
      this.dialog.show();
    },
    close: function () {
      this.dialog.hide();
    },
  },
});
