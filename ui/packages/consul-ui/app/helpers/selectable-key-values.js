import { helper } from '@ember/component/helper';
import { slugify } from 'consul-ui/helpers/slugify';
export const selectableKeyValues = function(params = [], hash = {}) {
  let selected;

  const items = params.map(function(item, i) {
    let key, value;
    switch (typeof item) {
      case 'string':
        key = slugify([item]);
        value = item;
        break;
      default:
        if (item.length > 1) {
          key = item[0];
          value = item[1];
        } else {
          key = slugify([item[0]]);
          value = item[0];
        }
        break;
    }
    const kv = {
      key: key,
      value: value,
    };
    switch (typeof hash.selected) {
      case 'string':
        if (hash.selected === item[0]) {
          selected = kv;
        }
        break;
      case 'number':
        if (hash.selected === i) {
          selected = kv;
        }
        break;
    }
    return kv;
  });
  return {
    items: items,
    selected: typeof selected === 'undefined' ? items[0] : selected,
  };
};
export default helper(selectableKeyValues);
