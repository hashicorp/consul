import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

const _isLegacy = function(token) {
  const rules = get(token, 'Rules');
  return get(token, 'Legacy') || (rules != null && rules.trim() != '');
};
export function isLegacy(params, hash) {
  const token = params[0];
  // is array like (RecordManager isn't an array)
  if (typeof token.length !== 'undefined') {
    return token.find(function(item) {
      return _isLegacy(item);
    });
  }
  return _isLegacy(token);
}

export default helper(isLegacy);
