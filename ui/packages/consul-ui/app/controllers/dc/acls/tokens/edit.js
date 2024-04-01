/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
export default Controller.extend({
  dom: service('dom'),
  builder: service('form'),
  isScoped: false,
  init: function () {
    this._super(...arguments);
    this.form = this.builder.form('token');
  },
  setProperties: function (model) {
    // essentially this replaces the data with changesets
    this._super(
      Object.keys(model).reduce((prev, key, i) => {
        switch (key) {
          case 'item':
            prev[key] = this.form.setData(prev[key]).getData();
            break;
        }
        return prev;
      }, model)
    );
  },
  actions: {
    change: function (e, value, item) {
      const event = this.dom.normalizeEvent(e, value);
      const form = this.form;
      try {
        form.handleEvent(event);
      } catch (err) {
        const target = event.target;
        switch (target.name) {
          default:
            throw err;
        }
      }
    },
  },
});
