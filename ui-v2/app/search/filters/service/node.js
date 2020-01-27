import { get } from '@ember/object';
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const term = s.toLowerCase();
    return (
      get(item, 'Node.Node')
        .toLowerCase()
        .indexOf(term) !== -1 ||
      get(item, 'Service.ID')
        .toLowerCase()
        .indexOf(term) !== -1 ||
      `${get(item, 'Service.Address')}:${get(item, 'Service.Port')}`.indexOf(term) !== -1
    );
  });
}
