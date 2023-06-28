/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Adapter from './application';
import isFolder from 'consul-ui/utils/isFolder';
import keyToArray from 'consul-ui/utils/keyToArray';
import { SLUG_KEY } from 'consul-ui/models/kv';

// TODO: Update to use this.formatDatacenter()
const API_KEYS_KEY = 'keys';

export default class KvAdapter extends Adapter {
  async requestForQuery(request, { dc, ns, partition, index, id, separator }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    const respond = await request`
      GET /v1/kv/${keyToArray(id)}?${{ [API_KEYS_KEY]: null, dc, separator }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
    await respond((headers, body) => delete headers['x-consul-index']);
    return respond;
  }

  async requestForQueryRecord(request, { dc, ns, partition, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    const respond = await request`
      GET /v1/kv/${keyToArray(id)}?${{ dc }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
    await respond((headers, body) => delete headers['x-consul-index']);
    return respond;
  }

  // TODO: Should we replace text/plain here with x-www-form-encoded? See
  // https://github.com/hashicorp/consul/issues/3804
  requestForCreateRecord(request, serialized, data) {
    const params = {
      dc: data.Datacenter,
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      PUT /v1/kv/${keyToArray(data[SLUG_KEY])}?${params}
      Content-Type: text/plain; charset=utf-8

      ${serialized}
    `;
  }

  requestForUpdateRecord(request, serialized, data) {
    const params = {
      dc: data.Datacenter,
      ns: data.Namespace,
      partition: data.Partition,
      flags: data.Flags,
    };
    return request`
      PUT /v1/kv/${keyToArray(data[SLUG_KEY])}?${params}
      Content-Type: text/plain; charset=utf-8

      ${serialized}
    `;
  }

  requestForDeleteRecord(request, serialized, data) {
    let recurse;
    if (isFolder(data[SLUG_KEY])) {
      recurse = null;
    }
    const params = {
      dc: data.Datacenter,
      ns: data.Namespace,
      partition: data.Partition,
      recurse,
    };
    return request`
      DELETE /v1/kv/${keyToArray(data[SLUG_KEY])}?${params}
    `;
  }
}
