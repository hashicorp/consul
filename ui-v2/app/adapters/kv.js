import Adapter, {
  REQUEST_CREATE,
  REQUEST_UPDATE,
  REQUEST_DELETE,
  DATACENTER_QUERY_PARAM as API_DATACENTER_KEY,
} from './application';
import isFolder from 'consul-ui/utils/isFolder';
import injectableRequestToJQueryAjaxHash from 'consul-ui/utils/injectableRequestToJQueryAjaxHash';
import { typeOf } from '@ember/utils';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';

import keyToArray from 'consul-ui/utils/keyToArray';
import removeNull from 'consul-ui/utils/remove-null';

import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/kv';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { PUT as HTTP_PUT, DELETE as HTTP_DELETE } from 'consul-ui/utils/http/method';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';

const API_KEYS_KEY = 'keys';
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
  decoder: service('atob'),
  urlForQuery: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    // append keys here otherwise query.keys will add an '='
    return this.appendURL('kv', keyToArray(query.id), {
      ...{
        [API_KEYS_KEY]: null,
      },
      ...this.cleanQuery(query),
    });
  },
  urlForQueryRecord: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return this.appendURL('kv', keyToArray(query.id), this.cleanQuery(query));
  },
  urlForCreateRecord: function(modelName, snapshot) {
    return this.appendURL('kv', keyToArray(snapshot.attr(SLUG_KEY)), {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    return this.appendURL('kv', keyToArray(snapshot.attr(SLUG_KEY)), {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    const query = {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    };
    if (isFolder(snapshot.attr(SLUG_KEY))) {
      query.recurse = null;
    }
    return this.appendURL('kv', keyToArray(snapshot.attr(SLUG_KEY)), query);
  },
  slugFromURL: function(url) {
    // keys don't follow the 'last part of the url' rule as they contain slashes
    return decodeURIComponent(
      url.pathname
        .split('/')
        .splice(3)
        .join('/')
    );
  },
  isQueryRecord: function(url, method) {
    return !url.searchParams.has(API_KEYS_KEY);
  },
  handleBatchResponse: function(url, response, primary, slug) {
    const dc = url.searchParams.get(API_DATACENTER_KEY) || '';
    return response.map((item, i, arr) => {
      return {
        [DATACENTER_KEY]: dc,
        [PRIMARY_KEY]: this.uidForURL(url, item),
        [SLUG_KEY]: item,
      };
    });
  },
  // handleSingleResponse: function(url, response, primary, slug) {
  //   return this._super(url, removeNull(response[0]), PRIMARY_KEY, SLUG_KEY);
  // },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    const method = requestData.method;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case response === true:
          response = this.handleBooleanResponse(url, response, PRIMARY_KEY, SLUG_KEY);
          break;
        case this.isQueryRecord(url, method):
          response = this.handleSingleResponse(url, removeNull(response[0]), PRIMARY_KEY, SLUG_KEY);
          break;
        default:
          response = this.handleBatchResponse(url, response, PRIMARY_KEY, SLUG_KEY);
      }
    }
    return this._super(status, headers, response, requestData);
  },
  dataForRequest: function(params) {
    const data = this._super(...arguments);
    let value = '';
    switch (params.requestType) {
      case REQUEST_UPDATE:
      case REQUEST_CREATE:
        value = data.kv.Value;
        if (typeof value === 'string') {
          return get(this, 'decoder').execute(value);
        }
        return null;
    }
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case REQUEST_DELETE:
        return HTTP_DELETE;
      case REQUEST_CREATE:
        return HTTP_PUT;
    }
    return this._super(...arguments);
  },
});
