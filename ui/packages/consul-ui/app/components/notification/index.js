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
    const $el = this.dom.element(`#${this.guid}`);
    const options = {
      timeout: 6000,
      extendedTimeout: 300,
      dom: $el.innerHTML,
    };
    if (this.sticky) {
      options.sticky = true;
    }
    $el.remove();
    this.notify.clearMessages();
    if (typeof this.after === 'function') {
      Promise.resolve(this.after())
        .catch(e => {
          if (e.name !== 'TransitionAborted') {
            throw e;
          }
        })
        .then(res => {
          this.notify.add(options);
        });
    } else {
      this.notify.add(options);
    }
  },
});
