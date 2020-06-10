import { helper } from '@ember/component/helper';

export default helper(function fromEntries(params /*, hash*/) {
  return Object.fromEntries(params);
});
