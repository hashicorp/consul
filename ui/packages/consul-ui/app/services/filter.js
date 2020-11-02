import Service from '@ember/service';

import service from 'consul-ui/filter/predicates/service';
import serviceInstance from 'consul-ui/filter/predicates/service-instance';
import node from 'consul-ui/filter/predicates/node';
import intention from 'consul-ui/filter/predicates/intention';
import token from 'consul-ui/filter/predicates/token';
import policy from 'consul-ui/filter/predicates/policy';

const predicates = {
  service: service(),
  serviceInstance: serviceInstance(),
  node: node(),
  intention: intention(),
  token: token(),
  policy: policy(),
};

export default class FilterService extends Service {
  predicate(type) {
    return predicates[type];
  }
}
