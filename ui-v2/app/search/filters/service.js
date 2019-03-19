import { get } from '@ember/object';
import ucfirst from 'consul-ui/utils/ucfirst';
const find = function(obj, term) {
  if (Array.isArray(obj)) {
    return obj.some(function(item) {
      return find(item, term);
    });
  }
  return obj.toLowerCase().indexOf(term) !== -1;
};
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const term = s.toLowerCase();
    let status;
    switch (true) {
      case term.indexOf('service:') === 0:
        return find(get(item, 'Name'), term.substr(8));
      case term.indexOf('tag:') === 0:
        return find(get(item, 'Tags') || [], term.substr(4));
      case term.indexOf('status:') === 0:
        status = term.substr(7);
        switch (term.substr(7)) {
          case 'warning':
          case 'critical':
          case 'passing':
            return get(item, `Checks${ucfirst(status)}`) > 0;
          default:
            return false;
        }
      default:
        return find(get(item, 'Name'), term) || find(get(item, 'Tags') || [], term);
    }
  });
}
