import Service from '@ember/service';
import setHelpers from 'mnemonist/set';

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

import filteredRole from 'consul-ui/search/filters/role';
import filteredPolicy from 'consul-ui/search/filters/policy';
// service instance
import nodeService from 'consul-ui/search/filters/node/service';
import serviceNode from 'consul-ui/search/filters/service/node';

import filterableFactory from 'consul-ui/utils/search/filterable';
const filterable = filterableFactory();
const searchables = {
  serviceInstance: serviceNode(filterable),
  nodeservice: nodeService(filterable),
  role: filteredRole(filterable),
  policy: filteredPolicy(filterable),
};
export const search = spec => {
  let possible = Object.keys(spec);
  return (term, options = {}) => {
    const actual = [
      ...setHelpers.intersection(new Set(possible), new Set(options.properties || possible)),
    ];
    return item => {
      return (
        typeof actual.find(key => {
          return spec[key](item, term);
        }) !== 'undefined'
      );
    };
  };
};
const predicates = {
  intention: search(intention),
  service: search(service),
  ['service-instance']: search(serviceInstance),
  ['upstream-instance']: upstreamInstance(),
  ['health-check']: search(healthCheck),
  node: search(node),
  kv: search(kv),
  acl: search(acl),
  token: search(token),
  role: search(role),
  policy: search(policy),
  nspace: search(nspace),
};
export default class SearchService extends Service {
  searchable(name) {
    return searchables[name];
  }
  predicate(name) {
    return predicates[name];
  }
}
