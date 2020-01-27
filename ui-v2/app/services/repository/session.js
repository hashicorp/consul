import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';

const modelName = 'session';
export default RepositoryService.extend({
  store: service('store'),
  getModelName: function() {
    return modelName;
  },
  findByNode: function(node, dc, nspace, configuration = {}) {
    const query = {
      id: node,
      dc: dc,
      ns: nspace,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store.query(this.getModelName(), query);
  },
  // TODO: Why Key? Probably should be findBySlug like the others
  findByKey: function(slug, dc, nspace) {
    return this.findBySlug(...arguments);
  },
});
