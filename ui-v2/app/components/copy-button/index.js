import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  clipboard: service('clipboard/os'),
  dom: service('dom'),
  tagName: '',
  init: function() {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
    this._listeners = this.dom.listeners();
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this._listeners.remove();
  },
  didInsertElement: function() {
    this._super(...arguments);
    const component = this;
    this._listeners.add(this.clipboard.execute(`#${this.guid}`), {
      success: function() {
        component.success(...arguments);
      },
      error: function() {
        component.error(...arguments);
      },
    });
  },
});
