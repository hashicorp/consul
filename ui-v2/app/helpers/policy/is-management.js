import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

export function policyIsManagement(params, hash) {
  return get(params[0], 'ID') === '00000000-0000-0000-0000-000000000000';
}

export default helper(policyIsManagement);
