import { helper } from '@ember/component/helper';

export default helper(function([str = '']) {
  return window.atob(str);
});
