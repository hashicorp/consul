import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';

import { SLUG_KEY } from 'consul-ui/models/role';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';

export default Adapter.extend({
  requestForQuery: function(request, { dc, index, id }) {
    return request`
      GET /v1/acl/roles?${{ dc }}

      ${{ index }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/acl/role/${id}?${{ dc }}

      ${{ index }}
    `;
  },
  requestForCreateRecord: function(request, serialized, data) {
    return request`
      PUT /v1/acl/role?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Namespace: serialized.Namespace,
        Policies: serialized.Policies,
        ServiceIdentities: serialized.ServiceIdentities,
      }}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    return request`
      PUT /v1/acl/role/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Namespace: serialized.Namespace,
        Policies: serialized.Policies,
        ServiceIdentities: serialized.ServiceIdentities,
      }}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    return request`
      DELETE /v1/acl/role/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
});
