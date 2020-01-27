import Adapter from './application';

import isFolder from 'consul-ui/utils/isFolder';
import keyToArray from 'consul-ui/utils/keyToArray';

import { SLUG_KEY } from 'consul-ui/models/kv';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { NSPACE_KEY } from 'consul-ui/models/nspace';

// TODO: Update to use this.formatDatacenter()
const API_KEYS_KEY = 'keys';
export default Adapter.extend({
  requestForQuery: function(request, { dc, ns, index, id, separator }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/kv/${keyToArray(id)}?${{ [API_KEYS_KEY]: null, dc, separator }}

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
      GET /v1/kv/${keyToArray(id)}?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
  // TODO: Should we replace text/plain here with x-www-form-encoded?
  // See https://github.com/hashicorp/consul/issues/3804
  requestForCreateRecord: function(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
    };
    return request`
      PUT /v1/kv/${keyToArray(data[SLUG_KEY])}?${params}
      Content-Type: text/plain; charset=utf-8

      ${serialized}
    `;
  },
  requestForUpdateRecord: function(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
    };
    return request`
      PUT /v1/kv/${keyToArray(data[SLUG_KEY])}?${params}
      Content-Type: text/plain; charset=utf-8

      ${serialized}
    `;
  },
  requestForDeleteRecord: function(request, serialized, data) {
    let recurse;
    if (isFolder(data[SLUG_KEY])) {
      recurse = null;
    }
    const params = {
      ...this.formatDatacenter(data[DATACENTER_KEY]),
      ...this.formatNspace(data[NSPACE_KEY]),
      recurse,
    };
    return request`
      DELETE /v1/kv/${keyToArray(data[SLUG_KEY])}?${params}
    `;
  },
});
