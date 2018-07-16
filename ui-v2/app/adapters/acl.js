import Adapter, {
  REQUEST_CREATE,
  REQUEST_UPDATE,
  REQUEST_DELETE,
  DATACENTER_QUERY_PARAM as API_DATACENTER_KEY,
} from './application';
import EmberError from '@ember/error';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/acl';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { PUT as HTTP_PUT } from 'consul-ui/utils/http/method';
import { OK as HTTP_OK, UNAUTHORIZED as HTTP_UNAUTHORIZED } from 'consul-ui/utils/http/status';

import makeAttrable from 'consul-ui/utils/makeAttrable';
const REQUEST_CLONE = 'cloneRecord';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    // https://www.consul.io/api/acl.html#list-acls
    return this.appendURL('acl/list', [], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    // https://www.consul.io/api/acl.html#read-acl-token
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return this.appendURL('acl/info', [query.id], this.cleanQuery(query));
  },
  urlForCreateRecord: function(modelName, snapshot) {
    // https://www.consul.io/api/acl.html#create-acl-token
    return this.appendURL('acl/create', [], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    // the id is in the payload, don't add it in here
    // https://www.consul.io/api/acl.html#update-acl-token
    return this.appendURL('acl/update', [], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    // https://www.consul.io/api/acl.html#delete-acl-token
    return this.appendURL('acl/destroy', [snapshot.attr(SLUG_KEY)], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForCloneRecord: function(modelName, snapshot) {
    // https://www.consul.io/api/acl.html#clone-acl-token
    return this.appendURL('acl/clone', [snapshot.attr(SLUG_KEY)], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForRequest: function({ type, snapshot, requestType }) {
    switch (requestType) {
      case 'cloneRecord':
        return this.urlForCloneRecord(type.modelName, snapshot);
    }
    return this._super(...arguments);
  },
  clone: function(store, modelClass, id, snapshot) {
    const params = {
      store: store,
      type: modelClass,
      id: id,
      snapshot: snapshot,
      requestType: 'cloneRecord',
    };
    // _requestFor is private... but these methods aren't, until they disappear..
    const request = {
      method: this.methodForRequest(params),
      url: this.urlForRequest(params),
      headers: this.headersForRequest(params),
      data: this.dataForRequest(params),
    };
    // TODO: private..
    return this._makeRequest(request);
  },
  dataForRequest: function(params) {
    const data = this._super(...arguments);
    switch (params.requestType) {
      case REQUEST_UPDATE:
      case REQUEST_CREATE:
        return data.acl;
    }
    return data;
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case REQUEST_DELETE:
      case REQUEST_CREATE:
      case REQUEST_CLONE:
        return HTTP_PUT;
    }
    return this._super(...arguments);
  },
  isCreateRecord: function(url, method) {
    return (
      url.pathname ===
      this.parseURL(this.urlForCreateRecord('acl', makeAttrable({ [DATACENTER_KEY]: '' }))).pathname
    );
  },
  isCloneRecord: function(url, method) {
    return (
      url.pathname ===
      this.parseURL(
        this.urlForCloneRecord(
          'acl',
          makeAttrable({ [SLUG_KEY]: this.slugFromURL(url), [DATACENTER_KEY]: '' })
        )
      ).pathname
    );
  },
  isUpdateRecord: function(url, method) {
    return (
      url.pathname ===
      this.parseURL(this.urlForUpdateRecord(null, 'acl', makeAttrable({ [DATACENTER_KEY]: '' })))
        .pathname
    );
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    const method = requestData.method;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case response === true:
          response = this.handleBooleanResponse(url, response, PRIMARY_KEY, SLUG_KEY);
          break;
        case this.isQueryRecord(url):
          response = this.handleSingleResponse(url, response[0], PRIMARY_KEY, SLUG_KEY);
          break;
        case this.isUpdateRecord(url, method):
        case this.isCreateRecord(url, method):
        case this.isCloneRecord(url, method):
          response = this.handleSingleResponse(url, response, PRIMARY_KEY, SLUG_KEY);
          break;
        default:
          response = this.handleBatchResponse(url, response, PRIMARY_KEY, SLUG_KEY);
      }
    } else if (status === HTTP_UNAUTHORIZED) {
      const e = new EmberError();
      e.code = status;
      e.message = payload;
      throw e;
    }
    return this._super(status, headers, response, requestData);
  },
});
