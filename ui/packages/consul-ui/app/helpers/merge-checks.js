import { helper } from '@ember/component/helper';
import mergeChecks from 'consul-ui/utils/merge-checks';

export default helper(function ([checks, exposed], hash) {
  return mergeChecks(checks, exposed);
});
