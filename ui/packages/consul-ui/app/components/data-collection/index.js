import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { computed, get, action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { sort } from '@ember/object/computed';
import { defineProperty } from '@ember/object';
import Fuse from 'fuse.js';

class FuzzySearch {
  constructor(items, options) {
    this.fuse = new Fuse(items, {
      shouldSort: false, // use our own sorting for the moment
      threshold: 0.4,
      keys: Object.keys(options.finders) || [],
      getFn(item, key) {
        return (options.finders[key](item) || []).toString();
      },
    });
  }

  search(s) {
    return this.fuse.search(s).map(item => item.item);
  }
}

class PredicateSearch {
  constructor(items, options) {
    this.items = items;
    this.options = options;
  }

  search(s) {
    const predicate = this.predicate(s);
    // Test the value of each key for each object against the regex
    // All that match are returned.
    return this.items.filter(item => {
      return Object.entries(this.options.finders).some(([key, finder]) => {
        const val = finder(item);
        if (Array.isArray(val)) {
          return val.some(predicate);
        } else {
          return predicate(val);
        }
      });
    });
  }
}
class ExactSearch extends PredicateSearch {
  predicate(s) {
    s = s.toLowerCase();
    return item =>
      item
        .toString()
        .toLowerCase()
        .indexOf(s) !== -1;
  }
}
class RegExpSearch extends PredicateSearch {
  predicate(s) {
    let regex;
    try {
      regex = new RegExp(s, 'i');
    } catch (e) {
      // Return a predicate that excludes everything; most likely due to an
      // eager search of an incomplete regex
      return () => false;
    }
    return item => regex.test(item);
  }
}
const searchables = {
  fuzzy: FuzzySearch,
  exact: ExactSearch,
  regex: RegExpSearch,
};
export default class DataCollectionComponent extends Component {
  @service('filter') filter;
  @service('sort') sort;
  @service('search') searchService;

  @tracked term = '';

  get kind() {
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

  get searchMethod() {
    return this.args.searchable || 'fuzzy';
  }

  get searchProperties() {
    return this.args.filters.searchproperties;
  }

  @computed('args{items,items.content}')
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

  @computed('filtered', 'searchTerm', 'searchable')
  get searched() {
    if (this.searchTerm === '') {
      return this.filtered;
    }
    return this.searchable.search(this.searchTerm);
  }

  @computed('kind', 'searchMethod', 'filtered', 'searchProperties')
  get searchable() {
    const searchable =
      typeof this.searchMethod === 'string' ? searchables[this.searchMethod] : this.args.searchable;
    return new searchable(this.filtered, {
      finders: Object.fromEntries(
        Object.entries(this.search.predicate(this.kind)).filter(([key, value]) => {
          return typeof this.searchProperties === 'undefined'
            ? true
            : this.searchProperties.includes(key);
        })
      ),
    });
  }

  @computed('kind', 'content', 'args.filters')
  get filtered() {
    if (typeof this.args.filters === 'undefined') {
      return this.content;
    }
    const predicate = this.filter.predicate(this.kind);
    if (typeof predicate === 'undefined') {
      return this.content;
    }
    return this.content.filter(predicate(this.args.filters));
  }

  @computed('kind', 'args.sort')
  get comparator() {
    if (typeof this.args.sort === 'undefined') {
      return [];
    }
    return this.sort.comparator(this.kind)(this.args.sort);
  }
}
