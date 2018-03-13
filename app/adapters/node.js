import Adapter from './application';
export default Adapter.extend({
  urlForQuery: function() {
    return this.urlForFindAll();
  },
  urlForFindAll: function() {
    return `/${this.namespace}/internal/ui/nodes`;
  },
  urlForFindRecord: function(id, modelName) {
    return `/${this.namespace}/internal/ui/node/${id}`;
  },
});
