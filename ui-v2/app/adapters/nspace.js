import Adapter from './application';
import { SLUG_KEY } from 'consul-ui/models/nspace';

// namespaces aren't categorized by datacenter, therefore no dc
export default Adapter.extend({
  requestForQuery: function(request, { index }) {
    return request`
      GET /v1/namespaces

      ${{ index }}
    `;
  },
  requestForQueryRecord: function(request, { index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an name');
    }
    return request`
      GET /v1/namespace/${id}

      ${{ index }}
    `;
  },
  requestForCreateRecord: function(request, serialized, data) {
    return request`
      PUT /v1/namespace/${data[SLUG_KEY]}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        ACLs: {
          PolicyDefaults: serialized.ACLs.PolicyDefaults.map(item => ({ ID: item.ID })),
          RoleDefaults: serialized.ACLs.RoleDefaults.map(item => ({ ID: item.ID })),
        },
      }}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    return request`
      PUT /v1/namespace/${data[SLUG_KEY]}

      ${{
        Description: serialized.Description,
        ACLs: {
          PolicyDefaults: serialized.ACLs.PolicyDefaults.map(item => ({ ID: item.ID })),
          RoleDefaults: serialized.ACLs.RoleDefaults.map(item => ({ ID: item.ID })),
        },
      }}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    return request`
      DELETE /v1/namespace/${data[SLUG_KEY]}
    `;
  },
});
