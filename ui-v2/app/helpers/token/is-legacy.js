import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

export function isLegacy(params, hash) {
  const token = params[0];
  return get(token, 'Legacy') || typeof get(token, 'Rules') !== 'undefined';
}

export default helper(isLegacy);
