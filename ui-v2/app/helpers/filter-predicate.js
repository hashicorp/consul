import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default Helper.extend({
  filter: service('filter'),
  compute([type, filters], hash) {
    return this.filter.predicate(type)(filters);
  },
});
