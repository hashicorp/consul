import { get } from '@ember/object';
export default function(checks, status) {
  let num = 0;
  switch (status) {
    case 'passing':
    case 'critical':
    case 'warning':
      num = get(checks.filterBy('Status', status), 'length');
      break;
    case '': // all
      num = 1;
      break;
  }
  return num > 0;
}
