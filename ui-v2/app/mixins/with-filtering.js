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
  setProperties: function() {
    this._super(...arguments);
    const query = get(this, 'queryParams');
    query.forEach((item, i, arr) => {
      const filters = get(this, 'filters');
      Object.keys(item).forEach(key => {
        set(filters, key, get(this, key));
      });
      set(this, 'filters', filters);
    });
  },
  actions: {
    filter: function(e) {
      const obj = toKeyValue(e.target);
      Object.keys(obj).forEach((key, i, arr) => {
        set(this, key, obj[key] != '' ? obj[key] : null);
      });
      set(this, 'filters', {
        ...this.get('filters'),
        ...obj,
      });
    },
  },
});
