import { get } from '@ember/object';
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const sLower = s.toLowerCase();
    return (
      get(item, 'Name')
        .toLowerCase()
        .indexOf(sLower) !== -1 ||
      get(item, 'ID')
        .toLowerCase()
        .indexOf(sLower) !== -1
    );
  });
}
