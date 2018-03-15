import Adapter from './application';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('acl/list');
  },
  urlForQueryRecord: function(query, modelName) {
    const acl = query.acl;
    delete query.acl;
    return this.appendURL('acl/info', [acl]);
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    return this.appendURL('acl/destroy', [id]);
  },
  urlForCreateRecord: function(modelName, snapshot) {
    return this.appendURL('acl/create');
  },
  urlForCloneRecord: function(modelName, snapshot) {
    return this.appendURL('acl/clone', [snapshot.attr('ID')]);
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    return this.appendURL('acl/update');
  },
  dataForRequest: function(params) {
    const data = this._super(...arguments);
    switch (params.requestType) {
      case 'updateRecord':
      case 'createRecord':
        return data.acl;
    }
    return data;
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case 'deleteRecord':
      case 'createRecord':
        return 'PUT';
      case 'queryRecord':
        if (params.query.clone) {
          return 'PUT';
        }
    }
    return this._super(...arguments);
  },
});
