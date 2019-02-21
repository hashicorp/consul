import Component from '@ember/component';
import { get, computed } from '@ember/object';
export default Component.extend({
  tagName: '',
  count: computed('value', function() {
    const value = get(this, 'value');
    if (Array.isArray(value)) {
      return value.length;
    }
    return value;
  }),
});
