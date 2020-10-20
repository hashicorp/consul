import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default Helper.extend({
  sort: service('sort'),
  compute([type, key], hash) {
    return this.sort.comparator(type)(key);
  },
});
