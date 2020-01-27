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
    }
    return this.store.query(this.getModelName(), query);
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
