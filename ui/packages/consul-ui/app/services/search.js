import Service from '@ember/service';

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
export default class SearchService extends Service {
  predicate(name) {
    return predicates[name];
  }
}
