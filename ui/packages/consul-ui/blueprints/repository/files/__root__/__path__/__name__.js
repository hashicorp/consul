import RepositoryService from 'consul-ui/services/repository';

const modelName = '<%= dasherizedModuleName %>';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
});
