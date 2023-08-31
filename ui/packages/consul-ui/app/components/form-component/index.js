import Component from '@ember/component';
import Slotted from 'block-slots';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
// match anything that isn't a [ or ] into multiple groups
const propRe = /([^[\]])+/g;
export default Component.extend(Slotted, {
  tagName: '',
  onreset: function () {},
  onchange: function () {},
  onerror: function () {},
  onsuccess: function () {},

  data: alias('form.data'),
  item: alias('form.data'),
  // TODO: Could probably alias item
  // or just use data/value instead

  dom: service('dom'),
  container: service('form'),

  actions: {
    change: function (e, value, item) {
      let event = this.dom.normalizeEvent(e, value);
      // currently form-components don't deal with deeply nested forms, only top level
      // we therefore grab the end of the nest off here,
      // so role[policy][Rules] will end up as policy[Rules]
      // but also policy[Rules] will end up as Rules
      // for now we look for a [ so we know whether this component is deeply
      // nested or not and we pass the name through as an optional argument to handleEvent
      // once this component handles deeply nested forms this can go
      const matches = [...event.target.name.matchAll(propRe)];
      const prop = matches[matches.length - 1][0];
      let name;
      if (prop.indexOf('[') === -1) {
        name = `${this.type}[${prop}]`;
      } else {
        name = prop;
      }
      this.form.handleEvent(event, name);
      this.onchange({ target: this });
    },
  },
});
