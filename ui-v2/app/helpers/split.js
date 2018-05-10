import { helper } from '@ember/component/helper';

export function split([array = [], separator = ','], hash) {
  return array.split(separator);
}

export default helper(split);
