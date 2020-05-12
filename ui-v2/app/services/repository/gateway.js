import RepositoryService from 'consul-ui/services/repository';

const modelName = 'gateway';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
});
