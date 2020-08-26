import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  datacenters: service('repository/dc'),
  nodes: service('repository/node'),
  node: service('repository/node'),
  gateways: service('repository/service'),
  services: service('repository/service'),
  service: service('repository/service'),
  ['service-instance']: service('repository/service-instance'),
  ['service-instances']: service('repository/service-instance'),
  proxies: service('repository/proxy'),
  ['proxy-instance']: service('repository/proxy'),
  ['discovery-chain']: service('repository/discovery-chain'),
  coordinates: service('repository/coordinate'),
  sessions: service('repository/session'),
  namespaces: service('repository/nspace'),
  intentions: service('repository/intention'),
  intention: service('repository/intention'),
  kv: service('repository/kv'),
  token: service('repository/token'),
  policies: service('repository/policy'),
  policy: service('repository/policy'),
  roles: service('repository/role'),
  oidc: service('repository/oidc-provider'),

  type: service('data-source/protocols/http/blocking'),

  source: function(src, configuration) {
    // TODO: Consider adding/requiring 'action': nspace, dc, model, action, ...rest
    const [, nspace, dc, model, ...rest] = src.split('/').map(decodeURIComponent);
    // nspaces can be filled, blank or *
    // so we might get urls like //dc/services
    let find;
    const repo = this[model];
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
    let method, slug;
    switch (model) {
      case 'datacenters':
      case 'namespaces':
        find = configuration => repo.findAll(configuration);
        break;
      case 'services':
      case 'nodes':
      case 'roles':
      case 'policies':
        find = configuration => repo.findAllByDatacenter(dc, nspace, configuration);
        break;
      case 'intentions':
        [method, ...slug] = rest;
        switch (method) {
          case 'for-service':
            find = configuration => repo.findByService(slug, dc, nspace, configuration);
            break;
          default:
            find = configuration => repo.findAllByDatacenter(dc, nspace, configuration);
            break;
        }
        break;
      case 'service-instances':
        [method, ...slug] = rest;
        switch (method) {
          case 'for-service':
            find = configuration => repo.findByService(slug, dc, nspace, configuration);
            break;
        }
        break;
      case 'coordinates':
        [method, ...slug] = rest;
        switch (method) {
          case 'for-node':
            find = configuration => repo.findAllByNode(slug, dc, configuration);
            break;
        }
        break;
      case 'proxies':
        [method, ...slug] = rest;
        switch (method) {
          case 'for-service':
            find = configuration => repo.findAllBySlug(slug, dc, nspace, configuration);
            break;
        }
        break;
      case 'gateways':
        [method, ...slug] = rest;
        switch (method) {
          case 'for-service':
            find = configuration => repo.findGatewayBySlug(slug, dc, nspace, configuration);
            break;
        }
        break;
      case 'sessions':
        [method, ...slug] = rest;
        switch (method) {
          case 'for-node':
            find = configuration => repo.findByNode(slug, dc, nspace, configuration);
            break;
        }
        break;
      case 'token':
        find = configuration => repo.self(rest[1], dc);
        break;
      case 'discovery-chain':
      case 'node':
        find = configuration => repo.findBySlug(rest[0], dc, nspace, configuration);
        break;
      case 'service-instance':
        // id, node, service
        find = configuration =>
          repo.findBySlug(rest[0], rest[1], rest[2], dc, nspace, configuration);
        break;
      case 'proxy-instance':
        // id, node, service
        find = configuration =>
          repo.findInstanceBySlug(rest[0], rest[1], rest[2], dc, nspace, configuration);
        break;
      case 'policy':
      case 'kv':
      case 'intention':
        slug = rest[0];
        if (slug) {
          find = configuration => repo.findBySlug(slug, dc, nspace, configuration);
        } else {
          find = configuration =>
            Promise.resolve(repo.create({ Datacenter: dc, Namespace: nspace }));
        }
        break;
      case 'oidc':
        [method, ...slug] = rest;
        switch (method) {
          case 'providers':
            find = configuration => repo.findAllByDatacenter(dc, nspace, configuration);
            break;
          case 'provider':
            find = configuration => repo.findBySlug(slug[0], dc, nspace);
            break;
          case 'authorize':
            find = configuration =>
              repo.authorize(slug[0], slug[1], slug[2], dc, nspace, configuration);
            break;
        }
        break;
    }
    return this.type.source(find, configuration);
  },
});
