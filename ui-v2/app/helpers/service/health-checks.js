import { helper } from '@ember/component/helper';

export function healthChecks([item], hash) {
  switch (true) {
    case item.ChecksCritical !== 0:
      return 'critical';
    case item.ChecksWarning !== 0:
      return 'warning';
    case item.ChecksPassing !== 0:
      return 'passing';
    default:
      return 'empty';
  }
}

export default helper(healthChecks);
