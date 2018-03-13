import Adapter from './application';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return `/${this.namespace}/acl/list`;
  },
  urlForQueryRecord: function(query, modelName) {
    const acl = query.acl;
    delete query.acl;
    if (query.clone) {
      delete query.clone;
      return `/${this.namespace}/acl/clone/${acl}`;
    }
    return `/${this.namespace}/acl/info/${acl}`;
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    return `/${this.namespace}/acl/destroy/${id}`;
  },
  urlForCreateRecord: function(modelName, snapshot) {
    return `/${this.namespace}/acl/create`;
  },
  urlForCloneRecord: function(modelName, snapshot) {
    // const id = snapshot.attr('ID');
    return `/${this.namespace}/acl/clone/${id}`;
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    return `/${this.namespace}/acl/update`;
  },
  dataForRequest: function(params) {
    // const { store, type, snapshot, requestType, query } = params;
    const data = this._super(...arguments);
    switch (params.requestType) {
      case 'updateRecord':
        return data.acl;
      case 'createRecord':
        return data.acl;
    }
    return data;
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case 'deleteRecord':
        return 'PUT';
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
