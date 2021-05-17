import Adapter from './application';
import { SLUG_KEY } from 'consul-ui/models/policy';
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
export default class PolicyAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, id }) {
    return request`
      GET /v1/acl/policies?${{ dc }}

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
      GET /v1/acl/policy/${id}?${{ dc }}

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
    };
    return request`
      PUT /v1/acl/policy?${params}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Rules: serialized.Rules,
        Datacenters: serialized.Datacenters,
        ...Namespace(serialized.Namespace),
      }}
    `;
  }

  requestForUpdateRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
    };
    return request`
      PUT /v1/acl/policy/${data[SLUG_KEY]}?${params}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Rules: serialized.Rules,
        Datacenters: serialized.Datacenters,
        ...Namespace(serialized.Namespace),
      }}
    `;
  }

  requestForDeleteRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
    };
    return request`
      DELETE /v1/acl/policy/${data[SLUG_KEY]}?${params}
    `;
  }
}
