import Mixin from '@ember/object/mixin';

import { computed, get, set } from '@ember/object';
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
    const filters = get(this, 'filters');
    return get(this, 'items').filter(item => {
      return this.filter(item, filters);
    });
  }),
  actions: {
    filter: function(e) {
      set(this, 'filters', {
        ...this.get('filters'),
        ...toKeyValue(e.target),
      });
    },
  },
});
