import { get } from '@ember/object';
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const term = s.toLowerCase();
    return (
      get(item, 'Service')
        .toLowerCase()
        .indexOf(term) !== -1 ||
      get(item, 'ID')
        .toLowerCase()
        .indexOf(term) !== -1 ||
      (get(item, 'Tags') || []).some(function(item) {
        return item.toLowerCase().indexOf(term) !== -1;
      }) ||
      get(item, 'Port')
        .toString()
        .toLowerCase()
        .indexOf(term) !== -1
    );
  });
}
