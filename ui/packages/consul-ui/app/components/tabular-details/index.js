/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
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
