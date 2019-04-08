import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { get } from '@ember/object';
import EmberError from '@ember/error';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/acl';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { OK as HTTP_OK, UNAUTHORIZED as HTTP_UNAUTHORIZED } from 'consul-ui/utils/http/status';

export default Adapter.extend({
  requestForQuery: function(request, { dc, index }) {
    // https://www.consul.io/api/acl.html#list-acls
    return request`
      GET /v1/acl/list?${{ dc }}

      ${{ index }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    // https://www.consul.io/api/acl.html#read-acl-token
    return request`
      GET /v1/acl/info/${id}?${{ dc }}

      ${{ index }}
    `;
  },
  requestForCreateRecord: function(request, data) {
    // https://www.consul.io/api/acl.html#create-acl-token
    return request`
      PUT /v1/acl/create?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForUpdateRecord: function(request, data) {
    // the id is in the data, don't add it in here
    // https://www.consul.io/api/acl.html#update-acl-token
    return request`
      PUT /v1/acl/update?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForDeleteRecord: function(request, data) {
    // https://www.consul.io/api/acl.html#delete-acl-token
    return request`
      PUT /v1/acl/destroy/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForCloneRecord: function(request, data) {
    // https://www.consul.io/api/acl.html#clone-acl-token
    return request`
      PUT /v1/acl/clone/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  clone: function(store, type, id, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForClone(request, unserialized), serialized)
      .then(respond => serializer.respondForQueryRecord(respond, unserialized));
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
