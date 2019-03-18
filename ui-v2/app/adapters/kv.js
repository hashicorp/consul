import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';

import isFolder from 'consul-ui/utils/isFolder';
import keyToArray from 'consul-ui/utils/keyToArray';

import { SLUG_KEY } from 'consul-ui/models/kv';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';

const API_KEYS_KEY = 'keys';
export default Adapter.extend({
  decoder: service('atob'),
  requestForQuery: function(request, { dc, index, id, separator }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/kv/${keyToArray(id)}?${{ [API_KEYS_KEY]: null, dc, index, separator }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/kv/${keyToArray(id)}?${{ dc, index }}
    `;
  },
  requestForCreateRecord: function(request, data) {
    return request`
      PUT /v1/kv/${keyToArray(data[SLUG_KEY])}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${typeof data.Value === 'string' ? get(this, 'decoder').execute(data.Value) : null}
    `;
  },
  requestForUpdateRecord: function(request, data) {
    return request`
      PUT /v1/kv/${keyToArray(data[SLUG_KEY])}?${{ [API_DATACENTER_KEY]: data[DATACENTER_KEY] }}

      ${typeof data.Value === 'string' ? get(this, 'decoder').execute(data.Value) : null}
    `;
  },
  requestForDeleteRecord: function(request, data) {
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
