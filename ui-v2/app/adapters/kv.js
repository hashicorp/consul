import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';

import isFolder from 'consul-ui/utils/isFolder';
import keyToArray from 'consul-ui/utils/keyToArray';

import { SLUG_KEY } from 'consul-ui/models/kv';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';

const API_KEYS_KEY = 'keys';
export default Adapter.extend({
  requestForQuery: function(request, { dc, index, id, separator }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/kv/${keyToArray(id)}?${{ [API_KEYS_KEY]: null, dc, separator }}

      ${{ index }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/kv/${keyToArray(id)}?${{ dc }}

      ${{ index }}
    `;
  },
  // TODO: Should we replace text/plain here with x-www-form-encoded?
  // See https://github.com/hashicorp/consul/issues/3804
  requestForCreateRecord: function(request, serialized, data) {
    return request`
      PUT /v1/kv/${keyToArray(data[SLUG_KEY])}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
      Content-Type: text/plain; charset=utf-8

      ${serialized}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    return request`
      PUT /v1/kv/${keyToArray(data[SLUG_KEY])}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}
      Content-Type: text/plain; charset=utf-8

      ${serialized}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    let recurse;
    if (isFolder(data[SLUG_KEY])) {
      recurse = null;
    }
    return request`
      DELETE /v1/kv/${keyToArray(data[SLUG_KEY])}?${{
      [API_DATACENTER_KEY]: data[DATACENTER_KEY],
      recurse,
    }}
    `;
  },
});
