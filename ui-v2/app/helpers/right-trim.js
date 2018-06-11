import { helper } from '@ember/component/helper';

export function rightTrim([str = '', search = ''], hash) {
  const pos = str.length - search.length;
  return str.indexOf(search) === pos ? str.substr(0, pos) : str;
}

export default helper(rightTrim);
