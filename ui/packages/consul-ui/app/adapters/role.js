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

export default class RoleAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, id }) {
    return request`
      GET /v1/acl/roles?${{ dc }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }

  requestForQueryRecord(request, { dc, ns, partition, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/acl/role/${id}?${{ dc }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }

  requestForCreateRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ns: serialized.Namespace,
      partition: serialized.Partition,
    };
    return request`
      PUT /v1/acl/role?${params}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Policies: serialized.Policies,
        ServiceIdentities: serialized.ServiceIdentities,
        NodeIdentities: serialized.NodeIdentities,
      }}
    `;
  }

  requestForUpdateRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ns: serialized.Namespace,
      partition: serialized.Partition,
    };
    return request`
      PUT /v1/acl/role/${data[SLUG_KEY]}?${params}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Policies: serialized.Policies,
        ServiceIdentities: serialized.ServiceIdentities,
        NodeIdentities: serialized.NodeIdentities,
      }}
    `;
  }

  requestForDeleteRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ns: serialized.Namespace,
      partition: serialized.Partition,
    };
    return request`
      DELETE /v1/acl/role/${data[SLUG_KEY]}?${params}
    `;
  }
}
