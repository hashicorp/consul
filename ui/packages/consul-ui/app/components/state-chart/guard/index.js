/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@ember/component';

export default Component.extend({
  tagName: '',
  didInsertElement: function () {
    this._super(...arguments);
    const component = this;
    this.chart.addGuard(this.name, function () {
      if (typeof component.cond === 'function') {
        return component.cond(...arguments);
      } else {
        return component.cond;
      }
    });
  },
  willDestroyElement: function () {
    this._super(...arguments);
    this.chart.removeGuard(this.name);
  },
});
