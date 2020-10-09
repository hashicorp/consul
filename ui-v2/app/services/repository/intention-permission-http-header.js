import RepositoryService from 'consul-ui/services/repository';

const modelName = 'intention-permission-http-header';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  create: function(obj = {}) {
    return this.store.createFragment(this.getModelName(), obj);
  },
  persist: function(item) {
    return item.execute();
  },
});
