import Adapter from './application';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('internal/ui/nodes');
  },
  urlForQueryRecord: function(query, modelName) {
    const id = query.id;
    delete query.id;
    return this.appendURL('internal/ui/node', [id]);
  },
  // handleResponse: function(status, headers, payload, requestData) {
  //   return this._super(status, headers, { nodes: payload }, requestData);
  // },
});
