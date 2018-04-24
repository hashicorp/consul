import Adapter from './application';
import isFolder from 'consul-ui/utils/isFolder';
import injectableRequestToJQueryAjaxHash from 'consul-ui/utils/injectableRequestToJQueryAjaxHash';
import { typeOf } from '@ember/utils';
import { get } from '@ember/object';

import makeAttrable from 'consul-ui/utils/makeAttrable';
import keyToArray from 'consul-ui/utils/keyToArray';

const PRIMARY_KEY = 'Key';
const DATACENTER_KEY = 'Datacenter';

const stringify = function(obj) {
  if (typeOf(obj) === 'string') {
    return obj;
  }
  return JSON.stringify(obj);
};
export default Adapter.extend({
  // There is no code path that can avoid the payload of a PUT request from
  // going via JSON.stringify.
  // Therefore a string payload of 'foobar' will always be encoded to '"foobar"'
  //
  // This means we have no other choice but rewriting the entire codepath or
  // overwriting the private `_requestToJQueryAjaxHash` method
  //
  // The `injectableRequestToJQueryAjaxHash` function makes the JSON object
  // injectable, meaning we can copy letter for letter the sourcecode of
  // `_requestToJQueryAjaxHash`, which means we can compare it with the original
  // private method within a test (`tests/unit/utils/injectableRequestToJQueryAjaxHash.js`).
  // This means, if `_requestToJQueryAjaxHash` changes between Ember versions
  // we will know about it

  _requestToJQueryAjaxHash: injectableRequestToJQueryAjaxHash({
    stringify: stringify,
  }),
  atob: window.atob,
  urlForQuery: function(query, modelName) {
    const parts = keyToArray(query.key);
    delete query.key;
    // append keys here otherwise query.keys will add an '='
    return this.appendURL('kv', parts, {
      ...{
        keys: null,
      },
      ...query,
    });
  },
  urlForQueryRecord: function(query, modelName) {
    const parts = keyToArray(query.key);
    delete query.key;
    return this.appendURL('kv', parts, query);
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
      dc: snapshot.attr(DATACENTER_KEY),
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
      // TODO: How reliable is this?
      // KV `Keys` and therefore id's can have spaces and therefore %20's in them
      const item = {
        [PRIMARY_KEY]: decodeURIComponent(
          url
            .split('/')
            .splice(3)
            .join('/')
        ),
        [DATACENTER_KEY]: '',
      }; // TODO: separator?
      // safest way to check this is a create?
      if (this.urlForCreateRecord(null, makeAttrable(item)).split('?')[0] === url) {
        response = item;
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
        const value = data.kv.Value;
        if (typeof value === 'string') {
          return get(this, 'atob')(value);
        }
        return null;
    }
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
