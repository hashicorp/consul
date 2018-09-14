import { helper } from '@ember/component/helper';

// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Number/toLocaleString
// the actual helper
const toLocaleString = function(num) {
  return num.toLocaleString();
};
// wrap this up to help with testing the unit
export function callIfType(type) {
  return function(cb) {
    return function(params, hash) {
      if (typeof params[0] !== type) {
        return params[0];
      }
      return cb(params[0]);
    };
  };
}
export default helper(callIfType('number')(toLocaleString));
