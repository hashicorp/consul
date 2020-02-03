/*eslint ember/closure-actions: "warn"*/
import Component from '@ember/component';
import { inject as service } from '@ember/service';
import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  expanded: false,
  keyboardAccess: true,
  onchange: function() {},
  init: function() {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
  },
  actions: {
    change: function(e) {
      if (!e.target.checked) {
        [...this.dom.elements(`[id^=popover-menu-${this.guid}]`)].forEach(function($item) {
          $item.checked = false;
        });
      }
      this.onchange(e);
    },
    // Temporary send here so we can send route actions
    // easily. It kind of makes sense that you'll want to perform
    // route actions from a popup menu for the moment
    send: function() {
      this.sendAction(...arguments);
    },
  },
});
