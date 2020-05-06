import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  datacenters: service('repository/dc'),
  namespaces: service('repository/nspace'),
  token: service('repository/token'),
  policies: service('repository/policy'),
  policy: service('repository/policy'),
  roles: service('repository/role'),
  oidc: service('repository/oidc-provider'),
  type: service('data-source/protocols/http/blocking'),
  source: function(src, configuration) {
    // TODO: Consider adding/requiring nspace, dc, model, action, ...rest
    const [, nspace, dc, model, ...rest] = src.split('/');
    // TODO: Consider throwing if we have an empty nspace or dc
    // we are going to use '*' for 'all' when we need that
    // and an empty value is the same as 'default'
    // reasoning for potentially doing it here is, uri's should
    // always be complete, they should never have things like '///model'
    let find;
    const repo = this[model];
    if (typeof repo.reconcile === 'function') {
      configuration.createEvent = function(result = {}, configuration) {
        const event = {
          type: 'message',
          data: result,
        };
        repo.reconcile(get(event, 'data.meta'));
        return event;
      };
    }
    let method, slug;
    switch (model) {
      case 'datacenters':
        find = configuration => repo.findAll(configuration);
        break;
      case 'namespaces':
        find = configuration => repo.findAll(configuration);
        break;
      case 'token':
        find = configuration => repo.self(rest[1], dc);
        break;
      case 'roles':
      case 'policies':
        find = configuration => repo.findAllByDatacenter(dc, nspace, configuration);
        break;
      case 'policy':
        find = configuration => repo.findBySlug(rest[0], dc, nspace, configuration);
        break;
      case 'oidc':
        [method, ...slug] = rest;
        switch (method) {
          case 'providers':
            find = configuration => repo.findAllByDatacenter(dc, nspace, configuration);
            break;
          case 'provider':
            find = configuration => {
              return repo.findBySlug(slug[0], dc, nspace);
            };
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
