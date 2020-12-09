import Service from '@ember/service';
import Fuse from 'fuse.js';

import intention from 'consul-ui/search/predicates/intention';
import upstreamInstance from 'consul-ui/search/predicates/upstream-instance';
import serviceInstance from 'consul-ui/search/predicates/service-instance';
import healthCheck from 'consul-ui/search/predicates/health-check';
import acl from 'consul-ui/search/predicates/acl';
import service from 'consul-ui/search/predicates/service';
import node from 'consul-ui/search/predicates/node';
import kv from 'consul-ui/search/predicates/kv';
import token from 'consul-ui/search/predicates/token';
import role from 'consul-ui/search/predicates/role';
import policy from 'consul-ui/search/predicates/policy';
import nspace from 'consul-ui/search/predicates/nspace';

const search = spec => spec;
const predicates = {
  intention: search(intention),
  service: search(service),
  ['service-instance']: search(serviceInstance),
  ['upstream-instance']: search(upstreamInstance),
  ['health-check']: search(healthCheck),
  node: search(node),
  kv: search(kv),
  acl: search(acl),
  token: search(token),
  role: search(role),
  policy: search(policy),
  nspace: search(nspace),
};

class FuzzySearch {
  constructor(items, options) {
    this.fuse = new Fuse(items, {
      includeMatches: true,

      shouldSort: false, // use our own sorting for the moment
      threshold: 0.4,
      keys: Object.keys(options.finders) || [],
      getFn(item, key) {
        return (options.finders[key[0]](item) || []).toString();
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
export default class SearchService extends Service {
  searchables = {
    fuzzy: FuzzySearch,
    exact: ExactSearch,
    regex: RegExpSearch,
  };
  predicate(name) {
    return predicates[name];
  }
}
