import { helper } from '@ember/component/helper';

export function healthChecks([items], hash) {
  let ChecksCritical = 0;
  let ChecksWarning = 0;
  let ChecksPassing = 0;

  items.forEach(item => {
    switch (item.Status) {
      case 'critical':
        ChecksCritical += 1;
        break;
      case 'warning':
        ChecksWarning += 1;
        break;
      case 'passing':
        ChecksPassing += 1;
        break;
      default:
        break;
    }
  });

  switch (true) {
    case ChecksCritical !== 0:
      return {
        status: 'failing',
        count: ChecksCritical,
      };
    case ChecksWarning !== 0:
      return {
        status: 'warning',
        count: ChecksWarning,
      };
    case ChecksPassing !== 0:
      return {
        status: 'passing',
        count: ChecksPassing,
      };
    default:
      return {
        status: 'empty',
      };
  }
}

export default helper(healthChecks);
