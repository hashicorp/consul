import { A } from '@ember/array';
import { helper } from '@ember/component/helper';
export function blockParams(params) {
  return A(params.slice());
}
export default helper(blockParams);
