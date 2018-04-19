import { helper } from '@ember/component/helper';

export function replace([str = '', search = '', replace = ''], hash) {
  return str.replace(search, replace);
}

export default helper(replace);
