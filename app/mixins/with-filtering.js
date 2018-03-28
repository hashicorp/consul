import Mixin from '@ember/object/mixin';

import { computed } from '@ember/object';
import { assign } from '@ember/polyfills';
const toKeyValue = function(el) {
  const key = el.name;
  let value = '';
  switch (el.type) {
    case 'radio':
    case 'search':
    case 'text':
      value = el.value;
      break;
  }
  return { [key]: value };
};
export default Mixin.create({
  filters: {},
  filtered: computed('items', 'filters', function() {
    const filters = this.get('filters');
    return this.get('items').filter(item => {
      return this.filter(item, filters);
    });
  }),
  actions: {
    filter: function(e) {
      this.set('filters', assign({}, this.get('filters'), toKeyValue(e.target)));
    },
  },
});
