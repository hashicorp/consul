import Component from '@ember/component';
import { set } from '@ember/object';

export default Component.extend({
  classNames: ['popover-select'],
  actions: {
    change: function(option, e) {
      set(this, 'selectedOption', option);
      this.onchange({ target: this });
    },
  },
});
