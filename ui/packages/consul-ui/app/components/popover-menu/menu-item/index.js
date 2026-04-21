/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

export default Component.extend({
  tagName: '',
  dom: service('dom'),

  formModalActive: false,

  get modalRoot() {
    return document.body;
  },

  init: function () {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
  },
  didInsertElement: function () {
    this._super(...arguments);
    this.menu.addSubmenu(this.guid);
  },
  didDestroyElement: function () {
    this._super(...arguments);
    this.menu.removeSubmenu(this.guid);
  },

  actions: {
    activateModal(modalName) {
      set(this, modalName, true);
    },

    deactivateModal(modalName) {
      set(this, modalName, false);
    },
  },
});
