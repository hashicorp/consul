import Adapter from './application';

export default Adapter.extend({
  urlForFindAll: function(id, modelName) {
    return `/${this.namespace}/catalog/datacenters`;
  },
});
