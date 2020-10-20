import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY } from 'consul-ui/models/proxy';
import { get, set } from '@ember/object';
const modelName = 'proxy';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  findAllBySlug: function(slug, dc, nspace, configuration = {}) {
    const query = {
      id: slug,
      ns: nspace,
      dc: dc,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query).then(items => {
      items.forEach(item => {
        // swap out the id for the services id
        // so we can then assign the proxy to it if it exists
        const id = JSON.parse(item.uid);
        id.pop();
        id.push(item.ServiceProxy.DestinationServiceID);
        const service = this.store.peekRecord('service-instance', JSON.stringify(id));
        if (service) {
          set(service, 'ProxyInstance', item);
        }
      });
      return items;
    });
  },
  findInstanceBySlug: function(id, node, slug, dc, nspace, configuration) {
    return this.findAllBySlug(slug, dc, nspace, configuration).then(function(items) {
      let res = {};
      if (get(items, 'length') > 0) {
        let instance = items.filterBy('ServiceProxy.DestinationServiceID', id).findBy('Node', node);
        if (instance) {
          res = instance;
        } else {
          instance = items.findBy('ServiceProxy.DestinationServiceName', slug);
          if (instance) {
            res = instance;
          }
        }
      }
      set(res, 'meta', get(items, 'meta'));
      return res;
    });
  },
});
