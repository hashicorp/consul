import { get, set } from '@ember/object';
export default function(arr, item, prop, value) {
  value = typeof value === 'undefined' ? get(item, prop) : value;
  const current = arr.findBy(prop, value);
  if (current) {
    // TODO: This is reliant on changeset?
    Object.keys(get(item, 'data')).forEach(function(prop) {
      set(current, prop, get(item, prop));
    });
    return current;
  }
}
