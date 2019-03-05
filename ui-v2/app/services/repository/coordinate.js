import RepositoryService from 'consul-ui/services/repository';

const modelName = 'coordinate';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
});
