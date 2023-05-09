import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

export default helper(([arrayLike = [], prop], hash) => {
  if (!Array.isArray(arrayLike)) {
    arrayLike = arrayLike.toArray();
  }
  return arrayLike.reduce((prev, item, i) => {
    prev[get(item, prop)] = item;
    return prev;
  }, {});
});
