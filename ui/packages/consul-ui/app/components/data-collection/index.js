import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { sort } from '@ember/object/computed';
import { defineProperty } from '@ember/object';

export default class DataCollectionComponent extends Component {
  @service('filter') filter;
  @service('sort') sort;
  @service('search') search;

  get type() {
    return this.args.type;
  }

  @computed('comparator', 'searched')
  get items() {
    // the ember sort computed accepts either:
    // 1. The name of a property (as a string) returning an array properties to sort by
    // 2. A function to use for sorting
    let comparator = 'comparator';
    if (typeof this.comparator === 'function') {
      comparator = this.comparator;
    }
    defineProperty(this, 'sorted', sort('searched', comparator));
    return this.sorted;
  }

  @computed('type', 'filtered', 'args.filters.searchproperties', 'args.search')
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

  @computed('type', 'args.items', 'args.filters')
  get filtered() {
    if (typeof this.args.filters === 'undefined') {
      return this.args.items;
    }
    const predicate = this.filter.predicate(this.type);
    if (typeof predicate === 'undefined') {
      return this.args.items;
    }
    return this.args.items.filter(predicate(this.args.filters));
  }

  @computed('type', 'args.sort')
  get comparator() {
    if (typeof this.args.sort === 'undefined') {
      return [];
    }
    return this.sort.comparator(this.type)(this.args.sort);
  }
}
