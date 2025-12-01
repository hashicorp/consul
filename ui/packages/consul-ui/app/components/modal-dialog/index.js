/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { set } from '@ember/object';
import Slotted from 'block-slots';
import { schedule } from '@ember/runloop';

export default Component.extend(Slotted, {
  tagName: '',
  onclose: function () {},
  onopen: function () {},
  isOpen: false,

  actions: {
    handleSetup: function (dialog) {
      this.dialog = dialog;
    },
    handleShow: function ({ target }) {
      set(this, 'isOpen', true);
      this.onopen({ target });
    },
    handleHide: function ({ target }) {
      schedule('afterRender', () => set(this, 'isOpen', false));
      this.onclose({ target });
    },
    open: function () {
      if (this.dialog) {
        this.dialog.show();
      }
    },
    close: function () {
      if (this.dialog) {
        this.dialog.hide();
      }
    },
  },
});
