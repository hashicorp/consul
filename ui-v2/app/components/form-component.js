import Component from '@ember/component';
import SlotsMixin from 'block-slots';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { alias } from '@ember/object/computed';
import WithListeners from 'consul-ui/mixins/with-listeners';
// match anything that isn't a [ or ] into multiple groups
const propRe = /([^\[\]])+/g;
export default Component.extend(WithListeners, SlotsMixin, {
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

  actions: {
    change: function(e, value, item) {
      let event = get(this, 'dom').normalizeEvent(e, value);
      const matches = [...event.target.name.matchAll(propRe)];
      const prop = matches[matches.length - 1][0];
      event = get(this, 'dom').normalizeEvent(
        `${get(this, 'type')}[${prop}]`,
        event.target.value,
        event.target
      );
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
