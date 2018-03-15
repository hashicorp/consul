import Adapter from './application';
export default Adapter.extend({
  urlForQuery: function() {
    return this.urlForFindAll();
  },
  urlForFindAll: function() {
    return this.appendURL('internal/ui/nodes');
  },
  urlForFindRecord: function(id, modelName) {
    return this.appendURL('internal/ui/node', [id]);
  },
});
