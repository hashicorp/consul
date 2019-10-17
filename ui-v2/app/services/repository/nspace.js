import RepositoryService from 'consul-ui/services/repository';

const modelName = 'nspace';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  findAll: function(configuration = {}) {
    const query = {};
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store.query(this.getModelName(), query);
  },
});
