import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import WithListeners from 'consul-ui/mixins/with-listeners';

// WithListeners is likely to be used, plus if you add it to the
// child component, then itis init method is called after the
// init method here, which means _listeners isn't available
// when reset is called
// TODO: Consider a more traditional mixin pattern for mixins...
// i.e. withListeners(Component) so you can call it in init or the method
// for now we've moved reset to didInsertElement
export default Component.extend(WithListeners, {
  dom: service('dom'),
  builder: service('form'),
  init: function() {
    this._super(...arguments);
    if (!get(this, 'form')) {
      set(this, 'form', get(this, 'builder').form(get(this, 'name')));
    }
  },
  didInsertElement: function() {
    this.reset({});
  },
  reset: function() {},
  actions: {
    change: function(e, value, item) {
      const event = get(this, 'dom').normalizeEvent(e, value);
      const form = get(this, 'form');
      try {
        form.handleEvent(event);
      } catch (err) {
        throw err;
      }
    },
  },
});
