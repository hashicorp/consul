import Component from '@ember/component';
import WithClickOutside from 'consul-ui/mixins/click-outside';
import { set } from '@ember/object';

export default Component.extend(WithClickOutside, {
  classNames: ['action-group'],
  onblur: function() {
    // set(this, 'checked', null);
  },
  onchange: function() {},
});
