/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';

const ENTER = 13;
export default Component.extend({
  tagName: '',
  keyboardAccess: false,
  dom: service('dom'),
  init: function () {
    this._super(...arguments);
    this.name = this.dom.guid(this);
  },
  actions: {
    keydown: function (e) {
      if (e.keyCode === ENTER) {
        e.target.dispatchEvent(new MouseEvent('click'));
      }
    },
    change: function (e) {
      this.onchange(
        this.dom.setEventTargetProperty(e, 'value', (value) => (value === '' ? undefined : value))
      );
    },
  },
});
