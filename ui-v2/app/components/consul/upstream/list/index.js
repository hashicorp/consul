import Component from '@ember/component';
import { get } from '@ember/object';

export default Component.extend({
  tagName: '',
  actions: {
    isLinkable: function(item) {
      return get(item, 'InstanceCount') > 0;
    },
  },
});
