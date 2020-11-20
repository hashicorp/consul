import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { sort } from '@ember/object/computed';

export default class ConsulAclList extends Component {
  @service('filter') filter;
  @service('sort') sort;
  @service('search') search;

  @sort('searched', 'comparator') sorted;

  get items() {
    return this.sorted;
  }

  get searched() {
    if (typeof this.args.search === 'undefined') {
      return this.filtered;
    }
    const predicate = this.search.predicate('acl');
    return this.filtered.filter(
      predicate(this.args.search)
    );
  }

  get filtered() {
    const predicate = this.filter.predicate('acl');
    return this.args.items.filter(predicate(this.args.filters));
  }

  get comparator() {
    return this.sort.comparator('acl')(this.args.sort);
  }
}
