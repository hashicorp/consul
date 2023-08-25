import { helper } from '@ember/component/helper';
import atob from 'consul-ui/utils/atob';
export default helper(function ([str = '']) {
  return atob(str);
});
