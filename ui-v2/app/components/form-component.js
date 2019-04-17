import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { alias } from '@ember/object/computed';
import WithListeners from 'consul-ui/mixins/with-listeners';

export default Component.extend(WithListeners, {
  onreset: function() {},
  onchange: function() {},
  onerror: function() {},
  onsuccess: function() {},

  data: alias('form.data'),
  item: alias('form.data'),
  // TODO: Could probably alias item
  // or just use data/value instead

  dom: service('dom'),
  container: service('form'),
  init: function() {
    this._super(...arguments);
  },

  actions: {
    change: function(e, value, item) {
      const event = get(this, 'dom').normalizeEvent(e, value);
      const form = get(this, 'form');
      try {
        form.handleEvent(event);
        this.onchange({ target: this });
      } catch (err) {
        throw err;
      }
    },
  },
});
