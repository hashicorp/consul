import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { SLUG_KEY } from 'consul-ui/models/acl';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';

// The old ACL system doesn't support the `ns=` query param
// TODO: Update to use this.formatDatacenter()
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
  requestForCreateRecord: function(request, serialized, data) {
    // https://www.consul.io/api/acl.html#create-acl-token
    return request`
      PUT /v1/acl/create?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${serialized}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    // the id is in the data, don't add it into the URL
    // https://www.consul.io/api/acl.html#update-acl-token
    return request`
      PUT /v1/acl/update?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${serialized}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    // https://www.consul.io/api/acl.html#delete-acl-token
    return request`
      PUT /v1/acl/destroy/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForCloneRecord: function(request, serialized, data) {
    // https://www.consul.io/api/acl.html#clone-acl-token
    return request`
      PUT /v1/acl/clone/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  clone: function(store, type, id, snapshot) {
    return this.request(
      function(adapter, request, serialized, unserialized) {
        return adapter.requestForCloneRecord(request, serialized, unserialized);
      },
      function(serializer, respond, serialized, unserialized) {
        return serializer.respondForCreateRecord(respond, serialized, unserialized);
      },
      snapshot,
      type.modelName
    );
  },
});
