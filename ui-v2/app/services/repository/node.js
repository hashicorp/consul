import RepositoryService from 'consul-ui/services/repository';

const modelName = 'node';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  findLeader: function(dc, configuration = {}) {
    const query = {
      dc: dc,
    };
    if (typeof configuration.refresh !== 'undefined') {
      query.uri = configuration.uri;
    }
    return this.store.queryLeader(this.getModelName(), query);
  },
});
