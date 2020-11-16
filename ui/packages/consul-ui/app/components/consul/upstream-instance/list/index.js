import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { sort } from '@ember/object/computed';

export default class ConsulUpstreamInstanceList extends Component {
  @service('sort') sort;
  @service('search') search;

  @sort('searched', 'comparator') sorted;

  get items() {
    return this.sorted;
  }
  get searched() {
    if (typeof this.args.search === 'undefined') {
      return this.args.items;
    }
    const predicate = this.search.predicate('upstream-instance');
    return this.args.items.filter(predicate(this.args.search));
  }
  get comparator() {
    return this.sort.comparator('upstream-instance')(this.args.sort);
  }
}
