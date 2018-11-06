import { get } from '@ember/object';
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const source = get(item, 'SourceName').toLowerCase();
    const destination = get(item, 'DestinationName').toLowerCase();
    const sLower = s.toLowerCase();
    const allLabel = 'All Services (*)'.toLowerCase();
    return (
      source.indexOf(sLower) !== -1 ||
      destination.indexOf(sLower) !== -1 ||
      (source === '*' && allLabel.indexOf(sLower) !== -1) ||
      (destination === '*' && allLabel.indexOf(sLower) !== -1)
    );
  });
}
