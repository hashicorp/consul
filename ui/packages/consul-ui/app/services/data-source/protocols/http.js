import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { getOwner } from '@ember/application';
import { match } from 'consul-ui/decorators/data-source';

export default class HttpService extends Service {
  @service('repository/dc')
  datacenters;

  @service('repository/node')
  nodes;

  @service('repository/node')
  node;

  @service('repository/node')
  leader;

  @service('repository/service')
  gateways;

  @service('repository/service')
  services;

  @service('repository/service')
  service;

  @service('repository/service-instance')
  'service-instance';

  @service('repository/service-instance')
  'proxy-service-instance';

  @service('repository/service-instance')
  'service-instances';

  @service('repository/proxy')
  proxies;

  @service('repository/proxy')
  'proxy-instance';

  @service('repository/discovery-chain')
  'discovery-chain';

  @service('repository/topology')
  topology;

  @service('repository/coordinate')
  coordinates;

  @service('repository/session')
  sessions;

  @service('repository/nspace')
  namespaces;

  @service('repository/intention')
  intentions;

  @service('repository/intention')
  intention;

  @service('repository/kv')
  kv;

  @service('repository/token')
  token;

  @service('repository/policy')
  policies;

  @service('repository/policy')
  policy;

  @service('repository/role')
  roles;

  @service('repository/oidc-provider')
  oidc;

  @service('repository/metrics')
  metrics;

  @service('data-source/protocols/http/blocking')
  type;

  source(src, configuration) {
    const [, , , model] = src.split('/');
    const repo = this[model];
    const route = match(src);
    const find = route.cb(route.params, getOwner(this));
    configuration.createEvent = function(result = {}, configuration) {
      const event = {
        type: 'message',
        data: result,
      };
      const meta = get(event, 'data.meta') || {};
      if (typeof meta.range === 'undefined') {
        repo.reconcile(meta);
      }
      return event;
    };
    return this.type.source(find, configuration);
  }
}
