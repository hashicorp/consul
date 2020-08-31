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
  create: function(obj) {
    delete obj.Namespace;
    return this._super(obj);
  },
  findByService: function(slug, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      nspace: nspace,
      filter: `SourceName == "${slug}" or DestinationName == "${slug}"`,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), {
      ...query,
    });
  },
});
