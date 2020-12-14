import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { computed, get, action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { sort } from '@ember/object/computed';
import { defineProperty } from '@ember/object';

export default class DataCollectionComponent extends Component {
  @service('filter') filter;
  @service('sort') sort;
  @service('search') searchService;

  @tracked term = '';

  get type() {
    return this.args.type;
  }

  @computed('term', 'args.search')
  get searchTerm() {
    return this.term || this.args.search || '';
  }

  @action
  search(term) {
    this.term = term;
    return this.items;
  }

  @computed('args{items,.items.content}')
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

  @computed('type', 'filtered', 'args.filters.searchproperties', 'searchTerm')
  get searched() {
    if (this.searchTerm === '') {
      return this.filtered;
    }
    const predicate = this.searchService.predicate(this.type);
    const options = {};
    if (typeof get(this, 'args.filters.searchproperties') !== 'undefined') {
      options.properties = this.args.filters.searchproperties;
    }
    return this.filtered.filter(predicate(this.searchTerm, options));
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
