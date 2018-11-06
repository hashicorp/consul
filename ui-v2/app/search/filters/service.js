import { get } from '@ember/object';
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const term = s.toLowerCase();
    return (
      get(item, 'Name')
        .toLowerCase()
        .indexOf(term) !== -1 ||
      (get(item, 'Tags') || []).some(function(item) {
        return item.toLowerCase().indexOf(term) !== -1;
      })
    );
  });
}
