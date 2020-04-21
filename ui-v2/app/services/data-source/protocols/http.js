import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  datacenters: service('repository/dc'),
  namespaces: service('repository/nspace'),
  token: service('repository/token'),
  type: service('data-source/protocols/http/blocking'),
  source: function(src, configuration) {
    const [, , /*nspace*/ dc, model, ...rest] = src.split('/');
    let find;
    const repo = this[model];
    if (typeof repo.reconcile === 'function') {
      configuration.createEvent = function(result = {}, configuration) {
        const event = {
          type: 'message',
          data: result,
        };
        if (repo.reconcile === 'function') {
          repo.reconcile(get(event, 'data.meta') || {});
        }
        return event;
      };
    }
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
    }
    return this.type.source(find, configuration);
  },
});
