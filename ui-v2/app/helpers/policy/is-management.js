import { helper } from '@ember/component/helper';
import { get } from '@ember/object';
const MANAGEMENT_ID = '00000000-0000-0000-0000-000000000001';
export function isManagement(params, hash) {
  return get(params[0], 'ID') === MANAGEMENT_ID;
}

export default helper(isManagement);
