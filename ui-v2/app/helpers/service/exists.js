import { helper } from '@ember/component/helper';

export function serviceExists([item], hash) {
  if (typeof item.InstanceCount === 'undefined') {
    return false;
  }

  return item.InstanceCount > 0;
}

export default helper(serviceExists);
