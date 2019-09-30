import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { inject as service } from '@ember/service';
import { SLUG_KEY } from 'consul-ui/models/token';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';

export default Adapter.extend({
  store: service('store'),

  requestForQuery: function(request, { dc, index, role, policy }) {
    return request`
      GET /v1/acl/tokens?${{ role, policy, dc }}

      ${{ index }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/acl/token/${id}?${{ dc }}

      ${{ index }}
    `;
  },
  requestForCreateRecord: function(request, serialized, data) {
    return request`
      PUT /v1/acl/token?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    // TODO: here we check data['Rules'] not serialized['Rules']
    // data.Rules is not undefined, and serialized.Rules is not null
    // revisit this at some point we should probably use serialized here

    // If a token has Rules, use the old API
    if (typeof data['Rules'] !== 'undefined') {
      // https://www.consul.io/api/acl/legacy.html#update-acl-token
      return request`
        PUT /v1/acl/update?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

        ${serialized}
      `;
    }
    return request`
      PUT /v1/acl/token/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${serialized}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    return request`
      DELETE /v1/acl/token/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForSelf: function(request, serialized, { dc, index, secret }) {
    // TODO: Change here and elsewhere to use Authorization Bearer Token
    // https://github.com/hashicorp/consul/pull/4502
    return request`
      GET /v1/acl/token/self?${{ dc }}
      X-Consul-Token: ${secret}

      ${{ index }}
    `;
  },
  requestForCloneRecord: function(request, serialized, unserialized) {
    // this uses snapshots
    const id = unserialized[SLUG_KEY];
    const dc = unserialized[DATACENTER_KEY];
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      PUT /v1/acl/token/${id}/clone?${{ [API_DATACENTER_KEY]: dc }}
    `;
  },
  // TODO: self doesn't get passed a snapshot right now
  // ideally it would just for consistency
  // thing is its probably not the same shape as a 'Token',
  // plus we can't create Snapshots as they are private, see services/store.js
  self: function(store, type, id, unserialized) {
    return this.request(
      function(adapter, request, serialized, unserialized) {
        return adapter.requestForSelf(request, serialized, unserialized);
      },
      function(serializer, respond, serialized, unserialized) {
        return serializer.respondForQueryRecord(respond, serialized, unserialized);
      },
      unserialized,
      type.modelName
    );
  },
  clone: function(store, type, id, snapshot) {
    return this.request(
      function(adapter, request, serialized, unserialized) {
        return adapter.requestForCloneRecord(request, serialized, unserialized);
      },
      function(serializer, respond, serialized, unserialized) {
        // here we just have to pass through the dc (like when querying)
        // eventually the id is created with this dc value and the id talen from the
        // json response of `acls/token/*/clone`
        return serializer.respondForQueryRecord(respond, {
          [API_DATACENTER_KEY]: unserialized[SLUG_KEY],
        });
      },
      snapshot,
      type.modelName
    );
  },
});
