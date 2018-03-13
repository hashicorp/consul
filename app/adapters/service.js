import Adapter from './application';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.urlForFindAll(); /* modelName, snapshot?? */
  },
  urlForFindAll: function(modelName, snapshot) {
    return `/${this.namespace}/internal/ui/services`;
  },
  urlForFindRecord: function(id, modelName) {
    return `/${this.namespace}/health/service/${id}`;
  },
});
