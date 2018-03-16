import Adapter from './application';
import createQueryString from 'consul-ui/utils/createQueryString';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('acl/list');
  },
  urlForQueryRecord: function(query, modelName) {
    const id = query.acl;
    delete query.acl;
    return this.appendURL('acl/info', [id]);
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    return this.appendURL('acl/destroy', [id], {
      dc: snapshot.attr('Datacenter'),
    });
  },
  urlForCreateRecord: function(modelName, snapshot) {
    return this.appendURL('acl/create', [], {
      dc: snapshot.attr('Datacenter'),
    });
  },
  urlForCloneRecord: function(modelName, snapshot) {
    return this.appendURL('acl/clone', [snapshot.attr('ID')]);
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    return this.appendURL('acl/update', [], {
      dc: snapshot.attr('Datacenter'),
    });
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
