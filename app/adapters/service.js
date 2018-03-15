import Adapter from './application';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    // be careful proxying down as urlForFindAll needs snapshot
    return this.urlForFindAll();
  },
  urlForFindAll: function(modelName, snapshot) {
    return this.appendURL('internal/ui/services');
  },
  urlForFindRecord: function(id, modelName) {
    return this.appendURL('health/service', [id]);
  },
});
