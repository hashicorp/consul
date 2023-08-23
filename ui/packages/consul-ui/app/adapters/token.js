import Adapter from './application';
import { inject as service } from '@ember/service';
import { SLUG_KEY } from 'consul-ui/models/token';

export default class TokenAdapter extends Adapter {
  @service('store') store;

  requestForQuery(request, { dc, ns, partition, index, role, policy }) {
    return request`
      GET /v1/acl/tokens?${{ role, policy, dc }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }

  async requestForQueryRecord(request, { dc, ns, partition, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    const respond = await request`
      GET /v1/acl/token/${id}?${{ dc }}
      Cache-Control: no-store

      ${{
        ns,
        partition,
        index,
      }}
    `;
    respond((headers, body) => delete headers['x-consul-index']);
    return respond;
  }

  requestForCreateRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data.Datacenter),
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      PUT /v1/acl/token?${params}

      ${{
        Description: serialized.Description,
        Policies: serialized.Policies,
        Roles: serialized.Roles,
        ServiceIdentities: serialized.ServiceIdentities,
        NodeIdentities: serialized.NodeIdentities,
        Local: serialized.Local,
      }}
    `;
  }

  requestForUpdateRecord(request, serialized, data) {
    // TODO: here we check data['Rules'] not serialized['Rules'] data.Rules is
    // not undefined, and serialized.Rules is not null revisit this at some
    // point we should probably use serialized here

    // If a token has Rules, use the old API
    if (typeof data['Rules'] !== 'undefined') {
      // https://www.consul.io/api/acl/legacy.html#update-acl-token
      // as we are using the old API we don't need to specify a nspace
      return request`
        PUT /v1/acl/update?${this.formatDatacenter(data.Datacenter)}

        ${serialized}
      `;
    }
    const params = {
      ...this.formatDatacenter(data.Datacenter),
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      PUT /v1/acl/token/${data[SLUG_KEY]}?${params}

      ${{
        Description: serialized.Description,
        Policies: serialized.Policies,
        Roles: serialized.Roles,
        ServiceIdentities: serialized.ServiceIdentities,
        NodeIdentities: serialized.NodeIdentities,
        Local: serialized.Local,
      }}
    `;
  }

  requestForDeleteRecord(request, serialized, data) {
    const params = {
      dc: data.Datacenter,
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      DELETE /v1/acl/token/${data[SLUG_KEY]}?${params}
    `;
  }

  requestForSelf(request, serialized, { dc, index, secret }) {
    // TODO: Change here and elsewhere to use Authorization Bearer Token
    // https://github.com/hashicorp/consul/pull/4502
    return request`
      GET /v1/acl/token/self?${{ dc }}
      X-Consul-Token: ${secret}
      Cache-Control: no-store

      ${{ index }}
    `;
  }

  requestForCloneRecord(request, serialized, data) {
    // this uses snapshots
    const id = data[SLUG_KEY];
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    const params = {
      dc: data.Datacenter,
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      PUT /v1/acl/token/${id}/clone?${params}
    `;
  }

  // TODO: self doesn't get passed a snapshot right now ideally it would just
  // for consistency thing is its probably not the same shape as a
  // 'Token', plus we can't create Snapshots as they are private, see
  // services/store.js
  self(store, type, id, unserialized) {
    return this.rpc(
      function(adapter, request, serialized, data) {
        return adapter.requestForSelf(request, serialized, data);
      },
      function(serializer, respond, serialized, data) {
        return serializer.respondForSelf(respond, serialized, data);
      },
      unserialized,
      type.modelName
    );
  }

  clone(store, type, id, snapshot) {
    return this.rpc(
      function(adapter, request, serialized, data) {
        return adapter.requestForCloneRecord(request, serialized, data);
      },
      (serializer, respond, serialized, data) => {
        // here we just have to pass through the dc (like when querying)
        // eventually the id is created with this dc value and the id taken from the
        // json response of `acls/token/*/clone`
        const params = {
          dc: data.Datacenter,
          ns: data.Namespace,
          partition: data.Partition,
        };
        return serializer.respondForQueryRecord(respond, params);
      },
      snapshot,
      type.modelName
    );
  }
}
