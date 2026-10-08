/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { set } from '@ember/object';

export default Component.extend({
  tagName: '',
  isPermissionModalOpen: false,

  openModal(permission) {
    if (permission !== undefined) {
      set(this, 'permission', permission);
    }
    set(this, 'isPermissionModalOpen', true);
  },

  closeModal() {
    set(this, 'isPermissionModalOpen', false);
    set(this, 'permission', undefined);
  },

  setPermission(permission) {
    set(this, 'permission', permission);
  },

  actions: {
    createNewLabel: function (template, term) {
      return template.replace(/{{term}}/g, term);
    },
    isUnique: function (items, term) {
      return !items.find((item) => item.Name === term);
    },
    add: function (name, changeset, value) {
      if (!(changeset.get(name) || []).includes(value)) {
        changeset.pushObject(name, value);
        changeset.validate();
      }
    },
    delete: function (name, changeset, value) {
      if ((changeset.get(name) || []).includes(value)) {
        changeset.removeObject(name, value);
        changeset.validate();
      }
    },
  },
});
