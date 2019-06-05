import { get } from '@ember/object';
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const sLower = s.toLowerCase();
    return (
      get(item, 'AccessorID')
        .toLowerCase()
        .indexOf(sLower) !== -1 ||
      // TODO: Check if Name can go, it was just for legacy
      get(item, 'Name')
        .toLowerCase()
        .indexOf(sLower) !== -1 ||
      get(item, 'Description')
        .toLowerCase()
        .indexOf(sLower) !== -1 ||
      (get(item, 'Policies') || []).some(function(item) {
        return item.Name.toLowerCase().indexOf(sLower) !== -1;
      }) ||
      (get(item, 'ServiceIdentities') || []).some(function(item) {
        return item.ServiceName.toLowerCase().indexOf(sLower) !== -1;
      }) ||
      (get(item, 'Roles') || []).some(function(item) {
        return item.Name.toLowerCase().indexOf(sLower) !== -1;
      })
    );
  });
}
