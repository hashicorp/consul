/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

/*eslint ember/closure-actions: "warn"*/
import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

export default Component.extend({
  tagName: '',
  dom: service('dom'),
  expanded: false,
  keyboardAccess: true,
  onchange: function () {},
  // TODO: this needs to be made dynamic/auto detect
  // for now use this to set left/right explicitly
  position: '',
  init: function () {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
    this.submenus = [];
  },
  actions: {
    addSubmenu: function (name) {
      set(this, 'submenus', this.submenus.concat(name));
    },
    removeSubmenu: function (name) {
      const pos = this.submenus.indexOf(name);
      if (pos !== -1) {
        this.submenus.splice(pos, 1);
        set(this, 'submenus', this.submenus);
      }
    },
    change: function (e) {
      if (!e.target.checked) {
        [...this.dom.elements(`[id^=popover-menu-${this.guid}]`)].forEach(function ($item) {
          $item.checked = false;
        });
      }
      this.onchange(e);
    },
  },
});
