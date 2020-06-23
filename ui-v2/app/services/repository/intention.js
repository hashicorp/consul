import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY } from 'consul-ui/models/intention';
const modelName = 'intention';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  shouldReconcile: function(method) {
    // TODO: This is to be switched out for something at an adapter level
    // so it works for both methods of interacting with data-sources
    switch (true) {
      case method === 'findByService' || method.indexOf('for-service'):
        return false;
    }
    return this._super(...arguments);
  },
  findByService: function(slug, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      nspace: nspace,
      filter: `SourceName == "${slug}" or DestinationName == "${slug}"`,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store.query(this.getModelName(), {
      ...query,
    });
  },
});
