import RepositoryService from 'consul-ui/services/repository';

const modelName = 'node';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  findByLeader: function(dc) {
    const query = {
      dc: dc,
    };
    return this.store.queryLeader(this.getModelName(), query);
  },
});
