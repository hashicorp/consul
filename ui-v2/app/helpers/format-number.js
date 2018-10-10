import { helper } from '@ember/component/helper';
import callIfType from 'consul-ui/utils/helpers/call-if-type';

// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Number/toLocaleString
export function toLocaleString(num, options) {
  // TODO: If I make locale configurable use an option
  // not mutiple arguments
  return num.toLocaleString(undefined, options);
}
export default helper(callIfType('number')(toLocaleString));
