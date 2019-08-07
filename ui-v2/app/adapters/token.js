import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
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
  // TODO: We should probably call this requestForCloneRecord
  requestForClone: function(request, serialized, unserialized) {
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
  // thing is its probably not the same shape as a 'Token'
  // we should probably at least pass a null id as the third argument
  self: function(store, type, unserialized) {
    const client = get(this, 'client');
    const adapter = this;
    const serializer = store.serializerFor(type.modelName);
    // const unserialized = snapshot.attributes();
    const serialized = unserialized; //serializer.serialize(snapshot, {});

    return client
      .request(function(request) {
        return adapter.requestForSelf(request, serialized, unserialized);
      })
      .catch(function(e) {
        return adapter.error(e);
      })
      .then(function(response) {
        // TODO: When HTTPAdapter:responder changes, this will also need to change
        return serializer.respondForQueryRecord(response, serialized, unserialized);
      });
    // TODO: Potentially add specific serializer errors here
    // .catch(function(e) {
    //   return Promise.reject(e);
    // });
  },
  clone: function(store, type, id, snapshot) {
    const client = get(this, 'client');
    const adapter = this;
    const serializer = store.serializerFor(type.modelName);
    const unserialized = snapshot.attributes();
    const serialized = serializer.serialize(snapshot, {});
    return client
      .request(function(request) {
        return adapter.requestForClone(request, serialized, unserialized);
      })
      .catch(function(e) {
        return adapter.error(e);
      })
      .then(function(response) {
        // TODO: When HTTPAdapter:responder changes, this will also need to change
        return serializer.respondForQueryRecord(response, {
          [API_DATACENTER_KEY]: unserialized[SLUG_KEY],
        });
      });
    // TODO: Potentially add specific serializer errors here
    // .catch(function(e) {
    //   return Promise.reject(e);
    // });
  },
});
