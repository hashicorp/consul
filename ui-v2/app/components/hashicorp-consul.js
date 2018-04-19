import Component from '@ember/component';
import { get, set } from '@ember/object';
export default Component.extend({
  isDropdownVisible: false,
  actions: {
    change: function(item) {
      set(this, 'isDropdownVisible', !get(this, 'isDropdownVisible'));
    },
  },
});
