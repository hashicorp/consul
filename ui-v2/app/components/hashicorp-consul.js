import Component from '@ember/component';
import { get, set } from '@ember/object';
export default Component.extend({
  isDropdownVisible: false,
  actions: {
    change: function(item) {
      if (get(this, 'dcs.length') > 0) {
        set(this, 'isDropdownVisible', !get(this, 'isDropdownVisible'));
      }
    },
  },
});
