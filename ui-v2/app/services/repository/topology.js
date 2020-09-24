import RepositoryService from 'consul-ui/services/repository';
const modelName = 'topology';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  findBySlug: function(id, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
      id: id,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.queryRecord(this.getModelName(), query);
  },
});
