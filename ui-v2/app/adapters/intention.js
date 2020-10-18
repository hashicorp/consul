import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { get } from '@ember/object';
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

    // get the information we need from the id, which has been previously encoded
    const [SourceNS, SourceName, DestinationNS, DestinationName] = id
      .split(':')
      .map(decodeURIComponent);

    return request`
      GET /v1/connect/intentions/exact?${{
        source: `${SourceNS}/${SourceName}`,
        destination: `${DestinationNS}/${DestinationName}`,
        dc: dc,
      }}
      Cache-Control: no-store

      ${{ index }}
    `;
  },
  requestForCreateRecord: function(request, serialized, data) {
    const body = {
      SourceNS: serialized.SourceNS,
      DestinationNS: serialized.DestinationNS,
      SourceName: serialized.SourceName,
      DestinationName: serialized.DestinationName,
      SourceType: serialized.SourceType,
      Meta: serialized.Meta,
      Description: serialized.Description,
    };

    // only send the Action if we have one
    if (get(serialized, 'Action.length')) {
      body.Action = serialized.Action;
    } else {
      // otherwise only send Permissions if we have them
      if (serialized.Permissions) {
        body.Permissions = serialized.Permissions;
      }
    }
    return request`
      PUT /v1/connect/intentions/exact?${{
        source: `${data.SourceNS}/${data.SourceName}`,
        destination: `${data.DestinationNS}/${data.DestinationName}`,
        [API_DATACENTER_KEY]: data[DATACENTER_KEY],
      }}

      ${body}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    // you can no longer save Destinations
    delete serialized.DestinationNS;
    delete serialized.DestinationName;
    return this.requestForCreateRecord(...arguments);
  },
  requestForDeleteRecord: function(request, serialized, data) {
    return request`
      DELETE /v1/connect/intentions/exact?${{
        source: `${data.SourceNS}/${data.SourceName}`,
        destination: `${data.DestinationNS}/${data.DestinationName}`,
        [API_DATACENTER_KEY]: data[DATACENTER_KEY],
      }}
    `;
  },
});
