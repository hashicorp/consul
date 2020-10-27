import RepositoryService from 'consul-ui/services/repository';
const modelName = 'intention-permission';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  create: function(obj = {}) {
    // intention-permission and intention-permission-http
    // are currently treated as one and the same
    return this.store.createFragment(this.getModelName(), {
      ...obj,
      HTTP: this.store.createFragment('intention-permission-http', obj.HTTP || {}),
    });
  },
  persist: function(item) {
    return item.execute();
  },
});
