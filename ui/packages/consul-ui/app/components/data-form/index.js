/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set, get } from '@ember/object';
import Slotted from 'block-slots';
import { isChangeset } from 'validated-changeset';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  builder: service('form'),
  create: false,
  ondelete: function () {
    return this.onsubmit(...arguments);
  },
  oncancel: function () {
    return this.onsubmit(...arguments);
  },
  onsubmit: function () {},
  onchange: function (e, form) {
    return form.handleEvent(e);
  },
  didReceiveAttrs: function () {
    this._super(...arguments);
    try {
      this.form = this.builder.form(this.type);
    } catch (e) {
      // passthrough
      // this lets us load view only data that doesn't have a form
    }
  },
  willRender: function () {
    this._super(...arguments);
    set(this, 'hasError', this._isRegistered('error'));
  },
  actions: {
    setData: function (data) {
      let changeset = data;
      // convert to a real changeset
      if (!isChangeset(data) && typeof this.form !== 'undefined') {
        changeset = this.form.setData(data).getData();
      }
      // mark as creating
      // and autofill the new record if required
      if (get(data, 'isNew')) {
        set(this, 'create', true);
        changeset = Object.entries(this.autofill || {}).reduce(function (prev, [key, value]) {
          set(prev, key, value);
          return prev;
        }, changeset);
      }
      set(this, 'data', changeset);
      return this.data;
    },
    change: function (e, value, item) {
      this.onchange(this.dom.normalizeEvent(e, value), this.form, this.form.getData());
    },
  },
});
