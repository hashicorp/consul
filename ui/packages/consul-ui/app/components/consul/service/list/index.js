import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { sort } from '@ember/object/computed';
import { action, defineProperty } from '@ember/object';

export default class ConsulServiceList extends Component {
  @service('filter') filter;
  @service('sort') sort;
  @service('search') search;

  type = 'service';

  get items() {
    let comparator = 'comparator';
    if (typeof this.comparator === 'function') {
      comparator = this.comparator;
    }
    defineProperty(this, 'sorted', sort('searched', comparator));
    return this.sorted;
  }

  get searched() {
    if (typeof this.args.search === 'undefined') {
      return this.filtered;
    }
    const predicate = this.search.predicate(this.type);
    const options = {};
    if (typeof this.args.filters.searchproperties !== 'undefined') {
      options.properties = this.args.filters.searchproperties;
    }
    return this.filtered.filter(predicate(this.args.search, options));
  }

  get filtered() {
    const predicate = this.filter.predicate(this.type);
    return this.args.items.filter(predicate(this.args.filters));
  }

  get comparator() {
    return this.sort.comparator('service')(this.args.sort);
  }

  @action
  isLinkable(item) {
    return item.InstanceCount > 0;
  }
}
