import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
// Intentions use SourceNS and DestinationNS properties for namespacing
// so we don't need to add the `?ns=` anywhere here

// TODO: Update to use this.formatDatacenter()
export default Adapter.extend({
  requestForQuery: function(request, { dc, filter, index, uri }) {
    return request`
      GET /v1/connect/intentions?${{ dc }}
      X-Request-ID: ${uri}${
      typeof filter !== 'undefined'
        ? `
      X-Range: ${filter}`
        : ``
    }

      ${{
        index,
        filter,
      }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    const [SourceNS, SourceName, DestinationNS, DestinationName] = id
      .split(':')
      .map(decodeURIComponent);
    return request`
      GET /v1/connect/intentions/exact?source=${SourceNS +
        '/' +
        SourceName}&destination=${DestinationNS + '/' + DestinationName}&${{ dc }}
      Cache-Control: no-store

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
      PUT /v1/connect/intentions/${data.LegacyID}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${{
        SourceNS: serialized.SourceNS,
        DestinationNS: serialized.DestinationNS,
        SourceName: serialized.SourceName,
        DestinationName: serialized.DestinationName,
        SourceType: serialized.SourceType,
        Action: serialized.Action,
        Meta: serialized.Meta,
        Description: serialized.Description,
      }}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    return request`
      DELETE /v1/connect/intentions/${data.LegacyID}?${{
      [API_DATACENTER_KEY]: data[DATACENTER_KEY],
    }}
    `;
  },
});
