import { helper } from '@ember/component/helper';

export function _default(params, hash) {
  if (params[0] === '' || typeof params[0] === 'undefined') {
    return params[1];
  }
  return params[0];
}

export default helper(_default);
