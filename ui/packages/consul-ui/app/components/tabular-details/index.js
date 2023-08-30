/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  dom: service('dom'),
  onchange: function () {},
  init: function () {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
  },
  actions: {
    click: function (e) {
      this.dom.clickFirstAnchor(e);
    },
    change: function (item, items, e) {
      this.onchange(e, item, items);
    },
  },
});
