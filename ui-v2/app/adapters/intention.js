import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { SLUG_KEY } from 'consul-ui/models/intention';
// Intentions use SourceNS and DestinationNS properties for namespacing
// so we don't need to add the `?ns=` anywhere here

// TODO: Update to use this.formatDatacenter()
export default Adapter.extend({
  requestForQuery: function(request, { dc, index, id }) {
    return request`
      GET /v1/connect/intentions?${{ dc }}

      ${{ index }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/connect/intentions/${id}?${{ dc }}

      ${{ index }}
    `;
  },
  requestForCreateRecord: function(request, serialized, data) {
    // TODO: need to make sure we remove dc
    return request`
      POST /v1/connect/intentions?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${{
        SourceNS: serialized.SourceNS,
        DestinationNS: serialized.DestinationNS,
        SourceName: serialized.SourceName,
        DestinationName: serialized.DestinationName,
        SourceType: serialized.SourceType,
        Action: serialized.Action,
        Description: serialized.Description,
      }}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    return request`
      PUT /v1/connect/intentions/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${{
        SourceNS: serialized.SourceNS,
        DestinationNS: serialized.DestinationNS,
        SourceName: serialized.SourceName,
        DestinationName: serialized.DestinationName,
        SourceType: serialized.SourceType,
        Action: serialized.Action,
        Description: serialized.Description,
      }}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    return request`
      DELETE /v1/connect/intentions/${data[SLUG_KEY]}?${{
      [API_DATACENTER_KEY]: data[DATACENTER_KEY],
    }}
    `;
  },
});
