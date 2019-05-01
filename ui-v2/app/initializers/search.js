import intention from 'consul-ui/search/filters/intention';
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

import filterableFactory from 'consul-ui/utils/search/filterable';
const filterable = filterableFactory();
export function initialize(application) {
  // Service-less injection using private properties at a per-project level
  const Builder = application.resolveRegistration('service:search');
  const searchables = {
    intention: intention(filterable),
    token: token(filterable),
    acl: acl(filterable),
    policy: policy(filterable),
    role: role(filterable),
    kv: kv(filterable),
    healthyNode: node(filterable),
    unhealthyNode: node(filterable),
    serviceInstance: serviceNode(filterable),
    nodeservice: nodeService(filterable),
    service: service(filterable),
  };
  Builder.reopen({
    searchable: function(name) {
      return searchables[name];
    },
  });
}

export default {
  initialize,
};
