import { get } from '@ember/object';
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const sLower = s.toLowerCase();
    return (
      get(item, 'Node')
        .toLowerCase()
        .indexOf(sLower) !== -1
    );
  });
}
