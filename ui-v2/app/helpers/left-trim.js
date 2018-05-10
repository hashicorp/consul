import { helper } from '@ember/component/helper';

export function leftTrim([str = '', search = ''], hash) {
  return str.indexOf(search) === 0 ? str.substr(search.length) : str;
}

export default helper(leftTrim);
