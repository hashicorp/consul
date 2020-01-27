import { helper } from '@ember/component/helper';
import { get } from '@ember/object';
const MANAGEMENT_ID = '00000000-0000-0000-0000-000000000001';
export function typeOf(params, hash) {
  const item = params[0];
  switch (true) {
    case get(item, 'ID') === MANAGEMENT_ID:
      return 'policy-management';
    case typeof get(item, 'template') === 'undefined':
      return 'role';
    case get(item, 'template') !== '':
      return 'policy-service-identity';
    default:
      return 'policy';
  }
}

export default helper(typeOf);
