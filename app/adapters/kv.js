import Adapter from './application';
import isFolder from 'consul-ui/utils/isFolder';
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
const PRIMARY_KEY = 'Key';
const DATACENTER_KEY = 'Datacenter';

export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    const parts = keyToArray(query.key);
    delete query.key;
    // append keys here otherwise query.keys will add an '='
    return this.appendURL('kv', parts, {
      keys: null,
    });
  },
  urlForQueryRecord: function(query, modelName) {
    const parts = keyToArray(query.key);
    delete query.key;
    return this.appendURL('kv', parts);
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    const query = {
      dc: snapshot.attr(DATACENTER_KEY),
    };
    if (isFolder(id)) {
      query.recurse = null;
    }
    return this.appendURL('kv', keyToArray(id), query);
  },
  urlForCreateRecord: function(modelName, snapshot) {
    return this.appendURL('kv', keyToArray(snapshot.attr('Key')), {
      dc: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    return this.appendURL('kv', keyToArray(id), {
      dc: snapshot.attr('Datacenter'),
    });
  },
  // isCreateRecord: function(parts) {
  //   const url = parts.splice(3).concat([""]).join('/');
  //   return this.urlForQueryRecord({id: ""}) === url;
  // },
  // isQueryRecord: function(parts) {
  //   const url = parts.slice(0, -1).concat([""]).join('/');
  //   return this.urlForQueryRecord({id: ""}) === url;
  // },
  // When you createRecord this seems to be the only way to retain the
  // 'id' or the 'Key' without overriding everything and resorting to private methods
  handleResponse: function(status, headers, payload, requestData) {
    // TODO: isCreateRecord..
    let response = payload;
    if (response === true) {
      // isBoolean? should error on false
      const url = requestData.url.split('?')[0];
      const kv = {
        [PRIMARY_KEY]: url
          .split('/')
          .splice(3)
          .join('/'),
        DATACENTER_KEY: '',
      }; // TODO: separator?
      // safest way to check this is a create?
      if (this.urlForCreateRecord(null, makeAttrable(kv)).split('?')[0] === url) {
        response = kv;
      }
    } else {
      // both query and queryRecord
    }
    return this._super(status, headers, response, requestData);
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
