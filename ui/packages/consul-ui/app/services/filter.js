import Service from '@ember/service';
import { andOr } from 'consul-ui/utils/filter';

import service from 'consul-ui/filter/predicates/service';
import serviceInstance from 'consul-ui/filter/predicates/service-instance';
import healthCheck from 'consul-ui/filter/predicates/health-check';
import node from 'consul-ui/filter/predicates/node';
import kv from 'consul-ui/filter/predicates/kv';
import intention from 'consul-ui/filter/predicates/intention';
import token from 'consul-ui/filter/predicates/token';
import policy from 'consul-ui/filter/predicates/policy';
import authMethod from 'consul-ui/filter/predicates/auth-method';
import peer from 'consul-ui/filter/predicates/peer';

const predicates = {
  service: andOr(service),
  ['service-instance']: andOr(serviceInstance),
  ['health-check']: andOr(healthCheck),
  ['auth-method']: andOr(authMethod),
  node: andOr(node),
  kv: andOr(kv),
  intention: andOr(intention),
  token: andOr(token),
  policy: andOr(policy),
  peer: andOr(peer),
};

export default class FilterService extends Service {
  predicate(type) {
    return predicates[type];
  }
}
