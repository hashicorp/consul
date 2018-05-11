import { get } from '@ember/object';
export default function(items) {
  return items.reduce(function(sum, check) {
    const status = get(check, 'Status');
    return status === 'critical' || status === 'warning' ? sum + 1 : sum;
  }, 0);
}
