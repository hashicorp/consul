import Service from '@ember/service';

import intention from 'consul-ui/search/predicates/intention';
import upstreamInstance from 'consul-ui/search/predicates/upstream-instance';

import token from 'consul-ui/search/filters/token';
import policy from 'consul-ui/search/filters/policy';
import role from 'consul-ui/search/filters/role';
import kv from 'consul-ui/search/filters/kv';
import acl from 'consul-ui/search/filters/acl';
import node from 'consul-ui/search/filters/node';
// service instance
import nodeService from 'consul-ui/search/filters/node/service';
import serviceNode from 'consul-ui/search/filters/service/node';
import service from 'consul-ui/search/filters/service';
import nspace from 'consul-ui/search/filters/nspace';

import filterableFactory from 'consul-ui/utils/search/filterable';
const filterable = filterableFactory();
const searchables = {
  token: token(filterable),
  acl: acl(filterable),
  policy: policy(filterable),
  role: role(filterable),
  kv: kv(filterable),
  node: node(filterable),
  serviceInstance: serviceNode(filterable),
  nodeservice: nodeService(filterable),
  service: service(filterable),
  nspace: nspace(filterable),
};
const predicates = {
  intention: intention(),
  ['upstream-instance']: upstreamInstance(),
};
export default class SearchService extends Service {
  searchable(name) {
    return searchables[name];
  }
  predicate(name) {
    return predicates[name];
  }
}
