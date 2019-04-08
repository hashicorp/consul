import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { inject as service } from '@ember/service';
import { SLUG_KEY } from 'consul-ui/models/token';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';

import WithPolicies from 'consul-ui/mixins/policy/as-many';
import WithRoles from 'consul-ui/mixins/role/as-many';

import { get } from '@ember/object';


export default Adapter.extend(WithRoles, WithPolicies, {
  store: service('store'),

  requestForQuery: function(request, { dc, index, policy }) {
    return request`
      GET /v1/acl/tokens?${{ policy, dc }}

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
  requestForCreateRecord: function(request, data) {
    return request`
      PUT /v1/acl/token?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForUpdateRecord: function(request, data) {
    // TODO: Serializer
    // If a token has Rules, use the old API
    if (typeof data['Rules'] !== 'undefined') {
      // TODO: need to clean up vars sent
      data['ID'] = data['SecretID'];
      data['Name'] = data['Description'];
      return request`
        PUT /v1/acl/update?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
      `;
    }
    if (typeof data['SecretID'] !== 'undefined') {
      delete data['SecretID'];
    }
    return request`
      PUT /v1/acl/token/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForDeleteRecord: function(request, data) {
    return request`
      DELETE /v1/acl/token/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForSelf: function(request, { dc, index, secret }) {
    // do we need dc and index here?
    return request`
      GET /v1/acl/token/self?${{ dc }}
      X-Consul-Token: ${secret}

      ${{ index }}
    `;
  },
  requestForClone: function(request, { dc, id }) {
    // this uses snapshots
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      PUT /v1/acl/token/${id}/clone?${{ dc }}
    `;
  },
  // TODO: self doesn't get passed a snapshot right now
  // ideally it would just for consistency
  // thing is its probably not the same shape as a 'Token'
  self: function(store, type, unserialized) {
    const serializer = store.serializerFor(type.modelName);
    // const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = unserialized; //serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForSelf(request, unserialized), serialized)
      .then(respond => serializer.respondForQueryRecord(respond, unserialized));
  },
  // TODO: Does id even need to be here now?
  clone: function(store, type, id, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForClone(request, unserialized), serialized)
      .then(respond => serializer.respondForQueryRecord(respond, unserialized));
  },
});
