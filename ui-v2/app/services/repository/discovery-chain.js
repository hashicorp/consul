import RepositoryService from 'consul-ui/services/repository';

const modelName = 'discovery-chain';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
});
