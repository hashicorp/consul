import Controller from '@ember/controller';
import { get, set } from '@ember/object';

export default Controller.extend({
  isDropdownVisible: false,
  actions: {
    change: function(item) {
      set(this, 'isDropdownVisible', !get(this, 'isDropdownVisible'));
    },
  },
});
