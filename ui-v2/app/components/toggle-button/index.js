import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  dom: service('dom'),
  tagName: '',
  checked: false,
  onchange: function() {},
  // TODO: reserved for the moment but we don't need it yet
  onblur: function() {},
  init: function() {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
    this._listeners = this.dom.listeners();
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this._listeners.remove();
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (this.checked) {
      this.addClickOutsideListener();
    } else {
      this._listeners.remove();
    }
  },
  addClickOutsideListener: function() {
    // default onblur event
    this._listeners.remove();
    this._listeners.add(this.dom.document(), 'click', e => {
      if (this.dom.isOutside(this.label, e.target)) {
        if (this.dom.isOutside(this.label.nextElementSibling, e.target)) {
          if (this.input.checked) {
            this.input.checked = false;
            // TODO: This should be an event
            this.onchange({ target: this.input });
          }
          this._listeners.remove();
        }
      }
    });
  },
  actions: {
    click: function(e) {
      // only preventDefault if the target isn't an external link
      // TODO: this should be changed for an explicit close
      if ((e.target.rel || '').indexOf('noopener') === -1) {
        e.preventDefault();
      }
      this.input.checked = !this.input.checked;
      // manually dispatched mouse events have a detail = 0
      // real mouse events have the number of click counts
      if (e.detail !== 0) {
        e.target.blur();
      }
      this.actions.change.apply(this, [e]);
    },
    change: function(e) {
      if (this.input.checked) {
        this.addClickOutsideListener();
      }
      // TODO: This should be an event
      this.onchange({ target: this.input });
    },
  },
});
