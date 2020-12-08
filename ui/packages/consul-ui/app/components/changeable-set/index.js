import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
  },
  willDestroyElement: function() {
    this._listeners.remove();
    this._super(...arguments);
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (this.items !== this.dispatcher.data) {
      this._listeners.remove();
      this._listeners.add(this.dispatcher, {
        change: e => set(this, 'items', e.target.data),
      });
      set(this, 'items', get(this.dispatcher, 'data'));
    }
    this.dispatcher.search(this.terms);
  },
});
