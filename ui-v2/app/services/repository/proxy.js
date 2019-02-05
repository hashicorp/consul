import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY } from 'consul-ui/models/proxy';
import { get } from '@ember/object';
const modelName = 'proxy';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  findAllBySlug: function(slug, dc, configuration = {}) {
    const query = {
      id: slug,
      dc: dc,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.get('store').query(this.getModelName(), query);
  },
  findInstanceBySlug: function(id, slug, dc, configuration) {
    return this.findAllBySlug(slug, dc, configuration).then(function(items) {
      if (get(items, 'length') > 0) {
        const instance = items.findBy('ServiceProxyDestination', id);
        if (instance) {
          return instance;
        }
      }
      return;
    });
  },
});
