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

  @computed('args.items', 'args.items.content')
  get content() {
    // TODO: Temporary little hack to ensure we detect DataSource proxy
    // objects but not any other special Ember Proxy object like ember-data
    // things. Remove this once we no longer need the Proxies
    if (this.args.items.dispatchEvent === 'function') {
      return this.args.items.content;
    }
    return this.args.items;
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

  @computed('type', 'content', 'args.filters')
  get filtered() {
    if (typeof this.args.filters === 'undefined') {
      return this.content;
    }
    const predicate = this.filter.predicate(this.type);
    if (typeof predicate === 'undefined') {
      return this.content;
    }
    return this.content.filter(predicate(this.args.filters));
  }

  @computed('type', 'args.sort')
  get comparator() {
    if (typeof this.args.sort === 'undefined') {
      return [];
    }
    return this.sort.comparator(this.type)(this.args.sort);
  }
}
