import { helper } from '@ember/component/helper';

export default helper(function substr([str = '', start = 0, length], hash) {
  return str.substr(start, length);
});
