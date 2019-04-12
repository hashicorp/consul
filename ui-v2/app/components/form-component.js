import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { alias } from '@ember/object/computed';
import WithListeners from 'consul-ui/mixins/with-listeners';

export default Component.extend(WithListeners, {
  onreset: function() {},
  onchange: function() {},
  onerror: function() {},
  onsuccess: function() {},

  uri: '',

  data: alias('form.data'),
  // TODO: Could probably alias item
  // or just use data/value instead

  dom: service('dom'),
  container: service('form'),

  init: function() {
    this._super(...arguments);
    if (!get(this, 'form')) {
      set(this, 'form', get(this, 'container').form(get(this, 'name')));
    }
    const form = get(this, 'form');
    if (!get(this, 'item')) {
      let item = form.getData();
      if (!item) {
        if (get(this, 'uri')) {
          // TODO: Support URI
        } else {
          this.reset();
        }
      } else {
        set(this, 'item', item);
        this.onreset({ target: this });
      }
    } else {
      // this.form.setData();
    }
    this.reset = this.reset.bind(this);
  },
  reset: function() {
    const form = get(this, 'form');
    const dc = get(this, 'dc');
    set(this, 'item', form.clear({ Datacenter: dc }));
    this.onreset({ target: this });
  },
  submit: function() {
    // TODO: Support submit
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
