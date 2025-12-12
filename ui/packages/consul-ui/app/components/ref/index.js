/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { set } from '@ember/object';

export default Component.extend({
  tagName: '',
  didReceiveAttrs: function () {
    this._super(...arguments);
    set(this.target, this.name, this.value);
  },
});
