import Component from '@ember/component';
import { set } from '@ember/object';

export default Component.extend({
  tagName: '',
  didReceiveAttrs: function() {
    set(this.target, this.name, this.value);
  },
});
