import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default Helper.extend({
  search: service('search'),
  compute([type, items], hash) {
    return this.search.searchable(type).add(items);
  },
});
