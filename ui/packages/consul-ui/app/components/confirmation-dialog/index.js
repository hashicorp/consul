/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/*eslint ember/closure-actions: "warn"*/
import Component from '@ember/component';

import Slotted from 'block-slots';
import { set } from '@ember/object';

export default Component.extend(Slotted, {
  tagName: '',
  message: 'Are you sure?',
  confirming: false,
  permanent: false,
  actions: {
    cancel: function () {
      set(this, 'confirming', false);
    },
    execute: function () {
      set(this, 'confirming', false);
      this.sendAction(...['actionName', ...this['arguments']]);
    },
    confirm: function () {
      const [action, ...args] = arguments;
      set(this, 'actionName', action);
      set(this, 'arguments', args);
      set(this, 'confirming', true);
    },
  },
});
