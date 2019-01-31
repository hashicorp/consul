import Component from '@ember/component';
import { subscribe } from 'consul-ui/utils/computed/purify';
export default Component.extend({
  tagName: '',
  count: subscribe('value', function(value) {
    if (Array.isArray(value)) {
      return value.length;
    }
    return value;
  }),
});
