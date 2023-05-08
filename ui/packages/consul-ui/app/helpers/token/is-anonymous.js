import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

const ANONYMOUS_ID = '00000000-0000-0000-0000-000000000002';
export function isAnonymous(params, hash) {
  return get(params[0], 'AccessorID') === ANONYMOUS_ID;
}
export default helper(isAnonymous);
