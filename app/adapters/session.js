import Adapter from './application';

export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    const id = query.node;
    delete query.node;
    return this.appendURL('session/node', [id]);
  },
  urlForQueryRecord: function(query, modelName) {
    const id = query.key;
    delete query.key;
    return this.appendURL('session/info', [id]);
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    return this.appendURL('session/destroy', [id]);
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case 'deleteRecord':
        return 'PUT';
    }
    return this._super(...arguments);
  },
});
