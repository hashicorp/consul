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
      GET /v1/acl/tokens?${{ dc, index, policy }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/acl/token/${id}?${{ dc, index }}
    `;
  },
  requestForCreateRecord: function(request, data) {
    // TODO: Serializer
    if (Array.isArray(data.Policies)) {
      data.Policies = data.Policies.filter(function(item) {
        // Just incase, don't save any policies that aren't saved
        return !get(item, 'isNew');
      }).map(function(item) {
        return {
          ID: get(item, 'ID'),
          Name: get(item, 'Name'),
        };
      });
    } else {
      delete data.Policies;
    }
    // TODO: need to make sure we remove dc
    return request`
      PUT /v1/acl/token?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${data}
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
        POST /v1/acl/update?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

        ${data}
      `;
    }
    if (typeof data['SecretID'] !== 'undefined') {
      delete data['SecretID'];
    }
    return request`
      POST /v1/acl/token/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${data}
    `;
  },
  requestForDeleteRecord: function(request, data) {
    return request`
      DELETE /v1/acl/token/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
  requestForSelf: function(request, headers, { dc, index, secret }) {
    // do we need dc and index here?
    return request`
      GET /v1/acl/token/self?${{ dc, index }}
      X-Consul-Token: ${secret}
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
  self: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const data = this.snapshotToJSON(store, snapshot, type);
    return get(this, 'client')
      .request(request => this.requestForSelf(request, data))
      .then(respond => serializer.respondForQueryRecord(respond, data));
  },
  // TODO: Does id even need to be here now?
  clone: function(store, type, id, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const data = this.snapshotToJSON(store, snapshot, type);
    return get(this, 'client')
      .request(request => this.requestForClone(request, data))
      .then(respond => serializer.respondForQueryRecord(respond, data));
  },
});
