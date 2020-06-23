import Component from '@ember/component';

import { inject as service } from '@ember/service';

export default Component.extend({
  tagName: '',
  notify: service('flashMessages'),
  dom: service('dom'),
  oncomplete: function() {},
  init: function() {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
  },
  didInsertElement: function() {
    const options = {
      timeout: 6000,
      extendedTimeout: 300,
      dom: this.dom.element(`#${this.guid}`).innerHTML,
    };
    if (typeof this.after === 'function') {
      Promise.resolve(this.after()).then(() => {
        this.notify.clearMessages();
        this.notify.add(options);
      });
    } else {
      this.notify.clearMessages();
      this.notify.add(options);
    }
  },
});
