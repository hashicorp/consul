import Adapter from './application';
const makeAttrable = function(obj) {
  return {
    attr: function(prop) {
      return obj[prop];
    },
  };
};
const keyToArray = function(key) {
  return (key === '/' ? '' : key).split('/');
};
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    const parts = keyToArray(query.key);
    delete query.key;
    return this.appendURL('kv', parts) + '?keys';
  },
  urlForQueryRecord: function(query, modelName) {
    const parts = keyToArray(query.key);
    delete query.key;
    return this.appendURL('kv', parts);
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    return this.appendURL('kv', [id]) + '?recurse';
  },
  urlForCreateRecord: function(modelName, snapshot) {
    return this.appendURL('kv', keyToArray(snapshot.attr('Key')));
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    return this.appendURL('kv', [id]);
  },
  handleResponse: function(status, headers, payload, requestData) {
    // TODO: isCreateRecord..
    if (payload === true) {
      const kv = {
        Key: requestData.url
          .split('/')
          .splice(3)
          .join('/'),
      }; // TODO: separator?
      // safest way to check this is a create?
      if (this.urlForCreateRecord(null, makeAttrable(kv)) === requestData.url) {
        payload = kv;
      }
    }
    return this._super(status, headers, payload, requestData);
  },
  dataForRequest: function(params) {
    const data = this._super(...arguments);
    switch (params.requestType) {
      case 'updateRecord':
      case 'createRecord':
        // this will never work as ember-data will ALWAYS JSON.stringify your payload,
        // thus adding "
        // TODO: only way around it is private...
        return data.kv.Value;
    }
    return data;
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case 'deleteRecord':
        return 'DELETE';
      case 'createRecord':
        return 'PUT';
    }
    return this._super(...arguments);
  },
});
