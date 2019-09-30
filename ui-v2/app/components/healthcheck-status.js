import Component from '@ember/component';
import { computed } from '@ember/object';
export default Component.extend({
  tagName: '',
  count: computed('value', function() {
    const value = this.value;
    if (Array.isArray(value)) {
      return value.length;
    }
    return value;
  }),
});
