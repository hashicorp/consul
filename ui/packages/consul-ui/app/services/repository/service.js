import RepositoryService from 'consul-ui/services/repository';
const modelName = 'service';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  findGatewayBySlug: function(slug, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
      gateway: slug,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  },
});
