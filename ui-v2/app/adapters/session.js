import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { SLUG_KEY } from 'consul-ui/models/session';

export default Adapter.extend({
  requestForQuery: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/session/node/${id}?${{ dc }}

      ${{ index }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/session/info/${id}?${{ dc }}

      ${{ index }}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    return request`
      PUT /v1/session/destroy/${data[SLUG_KEY]}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
    `;
  },
});
