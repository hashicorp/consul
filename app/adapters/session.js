import Adapter from './application';

export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    const id = query.node;
    delete query.node;
    return `/${this.namespace}/session/node/${id}`;
  },
  urlForQueryRecord: function(query, modelName) {
    const id = query.key;
    delete query.key;
    return `/${this.namespace}/session/info/${id}`;
  },
});
