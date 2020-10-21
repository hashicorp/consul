import { helper } from '@ember/component/helper';
import { get } from '@ember/object';
export function difference(params, hash) {
  return params[0].filter(function(item) {
    return !params[1].findBy('ID', get(item, 'ID'));
  });
}

export default helper(difference);
