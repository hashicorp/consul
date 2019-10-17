import Adapter from './application';
import { inject as service } from '@ember/service';

import { SLUG_KEY } from 'consul-ui/models/token';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { NSPACE_KEY } from 'consul-ui/models/nspace';

// TODO: Update to use this.formatDatacenter()
export default Adapter.extend({
  store: service('store'),

  requestForQuery: function(request, { dc, ns, index, role, policy }) {
    return request`
      GET /v1/acl/tokens?${{ role, policy, dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
  requestForQueryRecord: function(request, { dc, ns, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/acl/token/${id}?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
  requestForCreateRecord: function(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
    };
    return request`
      PUT /v1/acl/token?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${{
        Description: serialized.Description,
        Policies: serialized.Policies,
        Roles: serialized.Roles,
        ServiceIdentities: serialized.ServiceIdentities,
        Local: serialized.Local,
      }}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    // TODO: here we check data['Rules'] not serialized['Rules']
    // data.Rules is not undefined, and serialized.Rules is not null
    // revisit this at some point we should probably use serialized here

    // If a token has Rules, use the old API
    if (typeof data['Rules'] !== 'undefined') {
      // https://www.consul.io/api/acl/legacy.html#update-acl-token
      // as we are using the old API we don't need to specify a nspace
      return request`
        PUT /v1/acl/update?${this.formatDatacenter(data[DATACENTER_KEY])}

        ${serialized}
      `;
    }
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
    };
    return request`
      PUT /v1/acl/token/${data[SLUG_KEY]}?${params}

      ${{
        Description: serialized.Description,
        Policies: serialized.Policies,
        Roles: serialized.Roles,
        ServiceIdentities: serialized.ServiceIdentities,
        Local: serialized.Local,
      }}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
    };
    return request`
      DELETE /v1/acl/token/${data[SLUG_KEY]}?${params}
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
  requestForCloneRecord: function(request, serialized, data) {
    // this uses snapshots
    const id = data[SLUG_KEY];
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
    };
    return request`
      PUT /v1/acl/token/${id}/clone?${params}
    `;
  },
  // TODO: self doesn't get passed a snapshot right now
  // ideally it would just for consistency
  // thing is its probably not the same shape as a 'Token',
  // plus we can't create Snapshots as they are private, see services/store.js
  self: function(store, type, id, unserialized) {
    return this.request(
      function(adapter, request, serialized, data) {
        return adapter.requestForSelf(request, serialized, data);
      },
      function(serializer, respond, serialized, data) {
        return serializer.respondForQueryRecord(respond, serialized, data);
      },
      unserialized,
      type.modelName
    );
  },
  clone: function(store, type, id, snapshot) {
    return this.request(
      function(adapter, request, serialized, data) {
        return adapter.requestForCloneRecord(request, serialized, data);
      },
      (serializer, respond, serialized, data) => {
        // here we just have to pass through the dc (like when querying)
        // eventually the id is created with this dc value and the id taken from the
        // json response of `acls/token/*/clone`
        const params = {
          ...this.formatDatacenter(data[DATACENTER_KEY]),
          ...this.formatNspace(data[NSPACE_KEY]),
        };
        return serializer.respondForQueryRecord(respond, params);
      },
      snapshot,
      type.modelName
    );
  },
});
