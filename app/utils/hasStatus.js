export default function(checks, status) {
  let num = 0;
  switch (status) {
    case 'passing':
    case 'critical':
    case 'warning':
      num = checks.filterBy('Status', status).get('length');
      break;
    case '': // all
      num = 1;
      break;
  }
  return num > 0;
}
