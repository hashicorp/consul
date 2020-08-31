import Adapter from './application';

import { SLUG_KEY } from 'consul-ui/models/role';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { NSPACE_KEY } from 'consul-ui/models/nspace';

import { env } from 'consul-ui/env';
import nonEmptySet from 'consul-ui/utils/non-empty-set';

let Namespace;
if (env('CONSUL_NSPACES_ENABLED')) {
  Namespace = nonEmptySet('Namespace');
} else {
  Namespace = () => ({});
}
// TODO: Update to use this.formatDatacenter()
export default Adapter.extend({
  requestForQuery: function(request, { dc, ns, index, id }) {
    return request`
      GET /v1/acl/roles?${{ dc }}

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
      GET /v1/acl/role/${id}?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
  requestForCreateRecord: function(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
    };
    return request`
      PUT /v1/acl/role?${params}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Policies: serialized.Policies,
        ServiceIdentities: serialized.ServiceIdentities,
        NodeIdentities: serialized.NodeIdentities,
        ...Namespace(serialized.Namespace),
      }}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
    };
    return request`
      PUT /v1/acl/role/${data[SLUG_KEY]}?${params}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Policies: serialized.Policies,
        ServiceIdentities: serialized.ServiceIdentities,
        NodeIdentities: serialized.NodeIdentities,
        ...Namespace(serialized.Namespace),
      }}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
    };
    return request`
      DELETE /v1/acl/role/${data[SLUG_KEY]}?${params}
    `;
  },
});
