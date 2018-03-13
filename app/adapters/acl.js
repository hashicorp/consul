import Adapter from './application';
const token = '';
export default Adapter.extend({
  urlForQuery(query, modelName) {
    return `/${this.namespace}/acl/list?token=${token}`;
  },
  urlForQueryRecord(query, modelName) {
    const acl = query.acl;
    delete query.acl;
    if (query.clone) {
      delete query.clone;
      return `/${this.namespace}/acl/clone/${acl}?token=${token}`;
    }
    return `/${this.namespace}/acl/info/${acl}?token=${token}`;
  },
  urlForDeleteRecord(id, modelName, snapshot) {
    return `/${this.namespace}/acl/destroy/${id}?token=${token}`;
  },
  urlForCreateRecord(modelName, snapshot) {
    return `/${this.namespace}/acl/create?token=${token}`;
  },
  urlForCloneRecord(modelName, snapshot) {
    // const id = snapshot.attr('ID');
    return `/${this.namespace}/acl/clone/${id}?token=${token}`;
  },
  urlForUpdateRecord(id, modelName, snapshot) {
    return `/${this.namespace}/acl/update?token=${token}`;
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
  methodForRequest(params) {
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
